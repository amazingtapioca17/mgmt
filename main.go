package main

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/amazingtapioca17/mgmt/mgmtconn"
	"github.com/amazingtapioca17/mgmt/modules"

	customrib "github.com/amazingtapioca17/mgmt/table"
	"github.com/named-data/YaNFD/ndn"
	"github.com/named-data/YaNFD/ndn/lpv2"
	"github.com/named-data/YaNFD/ndn/mgmt"
	"github.com/named-data/YaNFD/ndn/tlv"
)

// type Thread struct {
// 	transport      *fake.FakeTransport
// 	localPrefix    *ndn.Name
// 	nonLocalPrefix *ndn.Name
// }

//	func MakeMgmtThread() *Thread {
//		m := new(Thread)
//		var err error
//		m.localPrefix, err = ndn.NameFromString("/localhost/nfd")
//		if err != nil {
//			core.LogFatal(m, "Unable to create name for management prefix: ", err)
//		}
//		m.nonLocalPrefix, err = ndn.NameFromString("/localhop/nfd")
//		if err != nil {
//			core.LogFatal(m, "Unable to create name for management prefix: ", err)
//		}
//		return m
//	}
var partialMessageStore map[uint64][][]byte

func mgmtConn() {
	// listen to incoming udp packets
	fmt.Println("listening for cleanup")
	udpServer, err := net.ListenPacket("udp", ":2000")
	if err != nil {
		return
	}
	defer udpServer.Close()

	for {
		buf := make([]byte, 1024)
		_, addr, err := udpServer.ReadFrom(buf)
		if err != nil {
			continue
		}
		go response(udpServer, addr, buf)
	}
}
func response(udpServer net.PacketConn, addr net.Addr, buf []byte) {
	buf = bytes.Trim(buf, "\x00")
	fibcommand := string(buf)
	command := strings.Split(fibcommand, ",")
	switch command[0] {
	case "clean":
		faceID, _ := strconv.Atoi(command[1])
		fmt.Println("cleaned", faceID)
		customrib.Rib.CleanUpFace(uint64(faceID))
	default:
	}
}
func main() {
	//go mgmtConn()
	// mgmtconn.Conn.Port = ":1080"
	// mgmtconn.Conn.Socket = "/tmp/fib.sock"
	mgmtconn.Conn.MakeMgmtConn()
	mgmtconn.Conn.Table = &customrib.Rib
	go mgmtconn.Conn.RunRead()
	manager := modules.MakeMgmtThread()
	manager.Run()
}

// func main1() {

// 	recvBuf := make([]byte, 8800)
// 	manager := MakeMgmtThread()
// 	manager.transport = fake.MakeFakeTransport()
// 	startPos := 0
// 	go manager.Run()
// 	for {
// 		readSize, err := manager.transport.Conn.Read(recvBuf[startPos:])
// 		startPos += readSize
// 		//fmt.Println(recvBuf)
// 		if err != nil {
// 			break
// 		}
// 		if startPos > tlv.MaxNDNPacketSize {
// 			continue
// 		}

// 		// Determine whether valid packet received
// 		tlvPos := 0
// 		for {
// 			if tlvPos >= startPos {
// 				startPos = 0
// 				break
// 			}

// 			_, _, tlvSize, err := tlv.DecodeTypeLength(recvBuf[tlvPos:])
// 			if err != nil {
// 				startPos = 0
// 				break
// 			} else if startPos >= tlvPos+tlvSize {
// 				processFrame(manager, recvBuf[tlvPos:tlvPos+tlvSize])
// 				tlvPos += tlvSize
// 			} else {
// 				if tlvPos > 0 {
// 					if startPos > tlvPos {
// 						// Move remaining data to beginning of buffer
// 						copy(recvBuf, recvBuf[tlvPos:startPos])
// 					}
// 					startPos -= tlvPos
// 				}
// 				break
// 			}
// 		}
// 	}
// }

// func processFrame(m *Thread, wire []byte) {
// 	m.transport.RecvQueue <- wire
// }

func processFrame1(wire []byte) {
	block, _, err := tlv.DecodeBlock(wire)

	if err != nil {
		return
	}
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

// func (m *Thread) Run() {
// 	for {
// 		block, pitToken, inFace := m.transport.Receive()
// 		if block == nil {
// 			// Indicates that internal face has quit, which means it's time for us to quit
// 			break
// 		}
// 		if block.Type() != tlv.Interest {
// 			core.LogWarn(m, "Dropping received non-Interest packet of type ", block.Type())
// 			continue
// 		}
// 		interest, err := ndn.DecodeInterest(block)
// 		if err != nil {
// 			continue
// 		}
// 		if interest.Name().Size() < m.localPrefix.Size()+2 { // Module + Verb
// 			continue
// 		}
// 		if !m.localPrefix.PrefixOf(interest.Name()) && !m.nonLocalPrefix.PrefixOf(interest.Name()) {
// 			continue
// 		}

// 		core.LogTrace(m, "Received management Interest ", interest.Name())
// 		var e error
// 		if e != nil {
// 			core.LogWarn("Failed to parse packet in LpPacket: %v", e)
// 		}
// 		// Dispatch interest based on name
// 		register(m, interest, pitToken, inFace)
// 	}
// }

// func register(m *Thread, interest *ndn.Interest, pitToken []byte, inFace uint64) {
// 	fmt.Println("registering")
// 	var response *mgmt.ControlResponse
// 	params := decodeControlParameters(interest)
// 	if params == nil {
// 		return
// 	}

// 	if params.Name == nil {
// 		return
// 	}

// 	faceID := inFace
// 	if params.FaceID != nil && *params.FaceID != 0 {
// 		faceID = *params.FaceID
// 	}

// 	origin := table.RouteOriginApp
// 	if params.Origin != nil {
// 		origin = *params.Origin
// 	}

// 	cost := uint64(0)
// 	if params.Cost != nil {
// 		cost = *params.Cost
// 	}

// 	flags := table.RouteFlagChildInherit
// 	if params.Flags != nil {
// 		flags = *params.Flags
// 	}

// 	expirationPeriod := (*time.Duration)(nil)
// 	if params.ExpirationPeriod != nil {
// 		expirationPeriod = new(time.Duration)
// 		*expirationPeriod = time.Duration(*params.ExpirationPeriod) * time.Millisecond
// 	}
// 	responseParams := mgmt.MakeControlParameters()
// 	responseParams.Name = params.Name
// 	responseParams.FaceID = new(uint64)
// 	*responseParams.FaceID = faceID
// 	responseParams.Origin = new(uint64)
// 	*responseParams.Origin = origin
// 	responseParams.Cost = new(uint64)
// 	*responseParams.Cost = cost
// 	responseParams.Flags = new(uint64)
// 	*responseParams.Flags = flags
// 	if expirationPeriod != nil {
// 		responseParams.ExpirationPeriod = new(uint64)
// 		*responseParams.ExpirationPeriod = uint64(expirationPeriod.Milliseconds())
// 	}
// 	responseParamsWire, err := responseParams.Encode()
// 	if err != nil {
// 		response = mgmt.MakeControlResponse(500, "Internal error", nil)
// 	} else {
// 		response = mgmt.MakeControlResponse(200, "OK", responseParamsWire)
// 	}
// 	sendResponse(m, response, interest, pitToken, inFace)
// }

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

// func sendResponse(m *Thread, response *mgmt.ControlResponse, interest *ndn.Interest, pitToken []byte, inFace uint64) {
// 	encodedResponse, err := response.Encode()
// 	if err != nil {
// 		return
// 	}
// 	encodedWire, err := encodedResponse.Wire()
// 	if err != nil {
// 		return
// 	}
// 	data := ndn.NewData(interest.Name(), encodedWire)

// 	encodedData, err := data.Encode()
// 	if err != nil {
// 		return
// 	}

// 	m.transport.Send(encodedData, pitToken, &inFace)
// }
