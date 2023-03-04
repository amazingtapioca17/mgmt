package mgmtconn

import (
	"bytes"
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
	a.unix.Write([]byte("list"))
	received := make([]byte, 8800)
	_, err := a.unix.Read(received)
	if err != nil {
		return []byte("")
	}
	return bytes.Trim(received, "\x00")
}
func (a *AckConn) ForwarderStatus() []byte {
	a.unix.Write([]byte("forwarderstatus"))
	received := make([]byte, 8800)
	_, err := a.unix.Read(received)
	if err != nil {
		return []byte("")
	}
	return bytes.Trim(received, "\x00")
}

func (a *AckConn) GetFaceId(faceID uint64) bool {
	a.unix.Write([]byte(fmt.Sprintf("faceid,%d", faceID)))
	received := make([]byte, 8800)
	_, err := a.unix.Read(received)
	if err != nil {
		return false
	}
	return string(received) == "ack"
}
