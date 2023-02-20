package main

import (
	"fmt"
	"net"
	"time"

	"github.com/named-data/YaNFD/core"
	"github.com/named-data/YaNFD/ndn"
	"github.com/named-data/YaNFD/ndn/lpv2"
	"github.com/named-data/YaNFD/ndn/mgmt"
	"github.com/named-data/YaNFD/ndn/tlv"
	"github.com/named-data/YaNFD/table"
)

const FACEID uint64 = 3

type FakeTransport struct {
	RecvQueue chan []byte // Contains pending packets sent to internal component
	SendQueue chan []byte // Contains pending packets sent by the internal component
	Conn      net.Conn
}

func MakeFakeTransport() *FakeTransport {
	t := new(FakeTransport)
	t.RecvQueue = make(chan []byte, 1024)
	t.SendQueue = make(chan []byte, 1024)
	t.Conn, _ = net.Dial("unix", "/run/nfd.sock")
	return t
}

type Thread struct {
	transport      *FakeTransport
	localPrefix    *ndn.Name
	nonLocalPrefix *ndn.Name
}

func MakeMgmtThread() *Thread {
	m := new(Thread)
	var err error
	m.localPrefix, err = ndn.NameFromString("/localhost/nfd")
	if err != nil {
		core.LogFatal(m, "Unable to create name for management prefix: ", err)
	}
	m.nonLocalPrefix, err = ndn.NameFromString("/localhop/nfd")
	if err != nil {
		core.LogFatal(m, "Unable to create name for management prefix: ", err)
	}
	return m
}

var partialMessageStore map[uint64][][]byte

func main() {

	recvBuf := make([]byte, 8800)
	manager := MakeMgmtThread()
	manager.transport = MakeFakeTransport()
	startPos := 0
	go manager.Run()
	for {
		readSize, err := manager.transport.Conn.Read(recvBuf[startPos:])
		startPos += readSize
		//fmt.Println(recvBuf)
		if err != nil {
			break
		}
		if startPos > tlv.MaxNDNPacketSize {
			continue
		}

		// Determine whether valid packet received
		tlvPos := 0
		for {
			if tlvPos >= startPos {
				startPos = 0
				break
			}

			_, _, tlvSize, err := tlv.DecodeTypeLength(recvBuf[tlvPos:])
			if err != nil {
				startPos = 0
				break
			} else if startPos >= tlvPos+tlvSize {
				processFrame(manager, recvBuf[tlvPos:tlvPos+tlvSize])
				tlvPos += tlvSize
			} else {
				if tlvPos > 0 {
					if startPos > tlvPos {
						// Move remaining data to beginning of buffer
						copy(recvBuf, recvBuf[tlvPos:startPos])
					}
					startPos -= tlvPos
				}
				break
			}
		}
	}
}

func (t *FakeTransport) Receive() (*tlv.Block, []byte, uint64) {
	shouldContinue := true
	// We need to use a for loop to silently ignore invalid packets
	for shouldContinue {
		select {
		case frame := <-t.RecvQueue:
			lpBlock, _, err := tlv.DecodeBlock(frame)
			if err != nil {
				core.LogWarn(t, "Unable to decode received block - DROP")
				continue
			}
			lpPacket, err := lpv2.DecodePacket(lpBlock)
			if err != nil {
				core.LogWarn(t, "Unable to decode received block - DROP")
				continue
			}
			if len(lpPacket.Fragment()) == 0 {
				core.LogWarn(t, "Received empty fragment - DROP")
				continue
			}

			block, _, err := tlv.DecodeBlock(lpPacket.Fragment())
			if err != nil {
				core.LogWarn(t, "Unable to decode received block - DROP")
				continue
			}
			return block, lpPacket.PitToken(), FACEID
		}
	}
	return nil, []byte{}, 0
}
func processFrame(m *Thread, wire []byte) {
	m.transport.RecvQueue <- wire
}

func processFrame1(wire []byte) {
	block, _, err := tlv.DecodeBlock(wire)

	if err != nil {
		return
	}
	//fmt.Println(block)
	// Now attempt to decode LpPacket from block
	frame, err := lpv2.DecodePacket(block)
	if err != nil {
		return
	}
	netPkt := frame.Fragment()
	netPacket := new(ndn.PendingPacket)
	netPacket.IncomingFaceID = new(uint64)
	*netPacket.IncomingFaceID = 1
	netPacket.Wire, _, err = tlv.DecodeBlock(netPkt)
	if err != nil {
		return
	}
	netPacket.CongestionMark = frame.CongestionMark()
	if len(frame.PitToken()) > 0 {
		netPacket.PitToken = make([]byte, len(frame.PitToken()))
		copy(netPacket.PitToken, frame.PitToken())
	}
	switch netPacket.Wire.Type() {
	case tlv.Interest:
		netPacket.NetPacket, err = ndn.DecodeInterest(netPacket.Wire)
		interest := netPacket.NetPacket.(*ndn.Interest)
		fmt.Println(interest)
	default:
		return
	}
}

func reassemble(frame *lpv2.Packet, baseSequence uint64, fragIndex uint64, fragCount uint64) []byte {
	_, hasSequence := partialMessageStore[baseSequence]
	if !hasSequence {
		// Create map entry
		partialMessageStore[baseSequence] = make([][]byte, fragCount)
	}

	// Insert into PartialMessageStore
	partialMessageStore[baseSequence][fragIndex] = make([]byte, len(frame.Fragment()))
	copy(partialMessageStore[baseSequence][fragIndex], frame.Fragment())

	// Determine whether it is time to reassemble
	receivedCount := 0
	receivedTotalLen := 0
	for _, fragment := range partialMessageStore[baseSequence] {
		if len(fragment) != 0 {
			receivedCount++
			receivedTotalLen += len(fragment)
		}
	}

	if receivedCount == len(partialMessageStore[baseSequence]) {
		// Time to reassemble!
		reassembled := make([]byte, receivedTotalLen)
		reassembledSize := 0
		for _, fragment := range partialMessageStore[baseSequence] {
			copy(reassembled[reassembledSize:], fragment)
			reassembledSize += len(fragment)
		}

		delete(partialMessageStore, baseSequence)
		return reassembled
	}

	return nil
}

func (m *Thread) Run() {
	for {
		block, pitToken, inFace := m.transport.Receive()
		if block == nil {
			// Indicates that internal face has quit, which means it's time for us to quit
			break
		}
		if block.Type() != tlv.Interest {
			core.LogWarn(m, "Dropping received non-Interest packet of type ", block.Type())
			continue
		}
		interest, err := ndn.DecodeInterest(block)
		if err != nil {
			continue
		}
		if interest.Name().Size() < m.localPrefix.Size()+2 { // Module + Verb
			continue
		}
		if !m.localPrefix.PrefixOf(interest.Name()) && !m.nonLocalPrefix.PrefixOf(interest.Name()) {
			continue
		}

		core.LogTrace(m, "Received management Interest ", interest.Name())
		var e error
		if e != nil {
			core.LogWarn("Failed to parse packet in LpPacket: %v", e)
		}
		// Dispatch interest based on name
		register(m, interest, pitToken, inFace)
	}
}

func register(m *Thread, interest *ndn.Interest, pitToken []byte, inFace uint64) {
	fmt.Println("registering")
	var response *mgmt.ControlResponse
	params := decodeControlParameters(interest)
	if params == nil {
		return
	}

	if params.Name == nil {
		return
	}

	faceID := inFace
	if params.FaceID != nil && *params.FaceID != 0 {
		faceID = *params.FaceID
	}

	origin := table.RouteOriginApp
	if params.Origin != nil {
		origin = *params.Origin
	}

	cost := uint64(0)
	if params.Cost != nil {
		cost = *params.Cost
	}

	flags := table.RouteFlagChildInherit
	if params.Flags != nil {
		flags = *params.Flags
	}

	expirationPeriod := (*time.Duration)(nil)
	if params.ExpirationPeriod != nil {
		expirationPeriod = new(time.Duration)
		*expirationPeriod = time.Duration(*params.ExpirationPeriod) * time.Millisecond
	}
	responseParams := mgmt.MakeControlParameters()
	responseParams.Name = params.Name
	responseParams.FaceID = new(uint64)
	*responseParams.FaceID = faceID
	responseParams.Origin = new(uint64)
	*responseParams.Origin = origin
	responseParams.Cost = new(uint64)
	*responseParams.Cost = cost
	responseParams.Flags = new(uint64)
	*responseParams.Flags = flags
	if expirationPeriod != nil {
		responseParams.ExpirationPeriod = new(uint64)
		*responseParams.ExpirationPeriod = uint64(expirationPeriod.Milliseconds())
	}
	responseParamsWire, err := responseParams.Encode()
	if err != nil {
		response = mgmt.MakeControlResponse(500, "Internal error", nil)
	} else {
		response = mgmt.MakeControlResponse(200, "OK", responseParamsWire)
	}
	sendResponse(m, response, interest, pitToken, inFace)
}

func decodeControlParameters(interest *ndn.Interest) *mgmt.ControlParameters {
	//fmt.Println("test")
	//fmt.Println(interest.Name().At(m.getManager().prefixLength() + 2))
	paramsRaw, _, err := tlv.DecodeBlock(interest.Name().At(4).Value())
	// fmt.Println("this is ndn style params raw")
	// fmt.Println(paramsRaw)
	if err != nil {
		return nil
	}
	params, err := mgmt.DecodeControlParameters(paramsRaw)
	if err != nil {
		return nil
	}
	return params
}

func sendResponse(m *Thread, response *mgmt.ControlResponse, interest *ndn.Interest, pitToken []byte, inFace uint64) {
	encodedResponse, err := response.Encode()
	if err != nil {
		return
	}
	encodedWire, err := encodedResponse.Wire()
	if err != nil {
		return
	}
	data := ndn.NewData(interest.Name(), encodedWire)

	encodedData, err := data.Encode()
	if err != nil {
		return
	}

	Send(m.transport, encodedData, pitToken, &inFace)
}

func Send(t *FakeTransport, block *tlv.Block, pitToken []byte, nextHopFaceID *uint64) {
	netWire, err := block.Wire()
	if err != nil {
		return
	}
	lpPacket := lpv2.NewPacket(netWire)
	if len(pitToken) > 0 {
		lpPacket.SetPitToken(pitToken)
	}
	if nextHopFaceID != nil {
		lpPacket.SetNextHopFaceID(*nextHopFaceID)
	}
	lpPacketWire, err := lpPacket.Encode()
	if err != nil {
		return
	}
	frame, err := lpPacketWire.Wire()
	if err != nil {
		return
	}
	fmt.Println("this is the control frame, ", frame)
	size, _ := t.Conn.Write(frame)
	fmt.Println(size)
}
