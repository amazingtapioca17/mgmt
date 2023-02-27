package transport

import (
	"fmt"
	"net"

	"github.com/named-data/YaNFD/core"
	"github.com/named-data/YaNFD/ndn/lpv2"
	"github.com/named-data/YaNFD/ndn/tlv"
)

type FakeTransport struct {
	RecvQueue chan []byte
	SendQueue chan []byte
	Conn      net.Conn
}

func MakeFakeTransport() *FakeTransport {
	t := new(FakeTransport)
	t.RecvQueue = make(chan []byte, 1024)
	t.SendQueue = make(chan []byte, 1024)
	t.Conn, _ = net.Dial("unix", "/run/nfd.sock")
	return t
}
func (t *FakeTransport) sendFrame(block []byte) {
	t.RecvQueue <- block
}

func (t *FakeTransport) RunReceive() {
	recvBuf := make([]byte, 8800)
	startPos := 0
	for {
		readSize, err := t.Conn.Read(recvBuf[startPos:])
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
				t.sendFrame(recvBuf[tlvPos : tlvPos+tlvSize])
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
			return block, lpPacket.PitToken(), *lpPacket.IncomingFaceID()
		}
	}
	return nil, []byte{}, 0
}

func (t *FakeTransport) Send(block *tlv.Block, pitToken []byte, nextHopFaceID *uint64) {
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
	// ack := check(*nextHopFaceID)
	// fmt.Println(ack, "got back an ack")
	t.Conn.Write(frame)

}

func check(faceID uint64) string {
	udpServer, err := net.ResolveUDPAddr("udp", ":1080")

	if err != nil {
		fmt.Println("ResolveUDPAddr failed:", err.Error())
	}

	conn, err := net.DialUDP("udp", nil, udpServer)
	if err != nil {
		fmt.Println("Listen failed:", err.Error())
	}

	//close the connection
	defer conn.Close()
	msg := fmt.Sprintf("register,/pp/poem,%d,0", faceID)
	_, err = conn.Write([]byte(msg))
	if err != nil {
		fmt.Println("Write data failed:", err.Error())
	}

	// buffer to get data
	received := make([]byte, 1024)
	_, err = conn.Read(received)
	if err != nil {
		fmt.Println("Read data failed:", err.Error())
	}
	return string(received)
}
