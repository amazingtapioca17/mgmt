package mgmtconn

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/amazingtapioca17/mgmt/ribinterface"
)

type AckConn struct {
	port   string
	socket string
	unix   net.Conn
	Table  ribinterface.RibInt
}

var AcksConn AckConn

func (a *AckConn) MakeMgmtConn(socket string) {
	a.socket = socket
	var err error
	a.unix, err = net.Dial("unixpacket", a.socket)
	fmt.Println("ack connected")
	if err != nil {
		fmt.Println("Error dialing socket:", err)
	}
}
func (a *AckConn) Conn() net.Conn {
	return a.unix
}

func (a *AckConn) GetAllFIBEntries() []byte {
	command := Message{
		Command: "list",
	}
	a.SendCommand(command)
	received := make([]byte, 8800)
	_, err := a.unix.Read(received)
	if err != nil {
		return []byte{}
	}
	msg := a.ParseResponse(received)
	return msg.Dataset
}
func (a *AckConn) ForwarderStatus() []byte {
	command := Message{
		Command: "forwarderstatus",
	}
	a.SendCommand(command)
	received := make([]byte, 8800)
	_, err := a.unix.Read(received)
	if err != nil {
		return []byte{}
	}
	msg := a.ParseResponse(received)
	return msg.Dataset
}

func (a *AckConn) Versions(strategy string) ([]uint64, bool) {
	command := Message{
		Command:  "versions",
		Strategy: strategy,
	}
	a.SendCommand(command)
	received := make([]byte, 8800)
	_, err := a.unix.Read(received)
	if err != nil {
		return []uint64{}, false
	}
	msg := a.ParseResponse(received)
	return msg.Versions, msg.Valid
}

func (a *AckConn) ListStrategy() []byte {
	command := Message{
		Command: "liststrategy",
	}
	a.SendCommand(command)
	received := make([]byte, 8800)
	_, err := a.unix.Read(received)
	if err != nil {
		return []byte{}
	}
	msg := a.ParseResponse(received)
	return msg.Dataset
}

func (a *AckConn) GetFaceId(faceID uint64) bool {
	command := Message{
		Command: "faceid",
		FaceID:  faceID,
	}
	a.SendCommand(command)
	received := make([]byte, 8800)
	_, err := a.unix.Read(received)
	if err != nil {
		return false
	}
	msg := a.ParseResponse(received)
	return msg.Valid
}

func (a *AckConn) ParseResponse(received []byte) Message {
	var msg Message
	err := json.Unmarshal(received, &msg)
	if err != nil {
		fmt.Println("error:", err)
	}
	return msg
}

func (a *AckConn) SendCommand(command Message) {
	b, err := json.Marshal(command)
	if err != nil {
		fmt.Println("error:", err)
	}
	_, err = a.unix.Write(b)
	if err != nil {
		fmt.Println("Write data failed:", err.Error())
	}
}
