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
	var commands = []string{"list"}
	b, err := json.Marshal(commands)
	if err != nil {
		fmt.Println("error:", err)
	}
	_, err = a.unix.Write([]byte(b))
	if err != nil {
		fmt.Println("Write data failed:", err.Error())
	}
	received := make([]byte, 8800)
	_, err = a.unix.Read(received)
	if err != nil {
		return []byte("")
	}
	msg := a.ParseResponse(received)
	return msg.Dataset
}
func (a *AckConn) ForwarderStatus() []byte {
	var commands = []string{"forwarderstatus"}
	b, err := json.Marshal(commands)
	if err != nil {
		fmt.Println("error:", err)
	}
	_, err = a.unix.Write([]byte(b))
	if err != nil {
		fmt.Println("Write data failed:", err.Error())
	}
	received := make([]byte, 8800)
	_, err = a.unix.Read(received)
	if err != nil {
		return []byte("")
	}
	msg := a.ParseResponse(received)
	return msg.Dataset
}

func (a *AckConn) Versions(version string) ([]uint64, bool) {
	var commands = []string{"versions", version}
	b, err := json.Marshal(commands)
	if err != nil {
		fmt.Println("error:", err)
	}
	_, err = a.unix.Write([]byte(b))
	if err != nil {
		fmt.Println("Write data failed:", err.Error())
	}
	received := make([]byte, 8800)
	_, err = a.unix.Read(received)
	if err != nil {
		return []uint64{}, false
	}
	msg := a.ParseResponse(received)
	return msg.Versions, msg.Valid
}

func (a *AckConn) ListStrategy() []byte {
	var commands = []string{"liststrategy"}
	b, err := json.Marshal(commands)
	if err != nil {
		fmt.Println("error:", err)
	}
	_, err = a.unix.Write([]byte(b))
	if err != nil {
		fmt.Println("Write data failed:", err.Error())
	}
	received := make([]byte, 8800)
	_, err = a.unix.Read(received)
	msg := a.ParseResponse(received)
	return msg.Dataset
}

func (a *AckConn) GetFaceId(faceID uint64) bool {
	var commands = []string{"faceid", fmt.Sprintf("%d", faceID)}
	b, err := json.Marshal(commands)
	if err != nil {
		fmt.Println("error:", err)
	}
	_, err = a.unix.Write([]byte(b))
	if err != nil {
		fmt.Println("Write data failed:", err.Error())
	}
	received := make([]byte, 8800)
	_, err = a.unix.Read(received)
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
