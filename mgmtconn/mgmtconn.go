package mgmtconn

import (
	"bytes"
	"fmt"
	"net"
	"strconv"

	"github.com/amazingtapioca17/mgmt/ribinterface"
	"github.com/named-data/YaNFD/ndn"
)

type MgmtConn struct {
	port   string
	socket string
	unix   net.Conn
	Table  ribinterface.RibInt
}

var Conn MgmtConn

func (m *MgmtConn) MakeMgmtConn(socket string) {
	m.socket = socket
	var err error
	m.unix, err = net.Dial("unixpacket", m.socket)
	if err != nil {
		fmt.Println("Error dialing socket:", err)
	}
}
func (m *MgmtConn) Conn() net.Conn {
	return m.unix
}
func (m *MgmtConn) ClearNextHops(name *ndn.Name) {
	fmt.Println("clear")
	var err error
	msg := fmt.Sprintf("clear,%s", name)
	_, err = m.unix.Write([]byte(msg))
	if err != nil {
		fmt.Println("Write data failed:", err.Error())
	}
}

func (m *MgmtConn) RemoveNextHop(name *ndn.Name, faceID uint64) {
	fmt.Println("remove")
	var err error
	msg := fmt.Sprintf("insert,%s,%d", name, faceID)
	_, err = m.unix.Write([]byte(msg))
	if err != nil {
		fmt.Println("Write data failed:", err.Error())
	}
}

func (m *MgmtConn) InsertNextHop(name *ndn.Name, faceID uint64, cost uint64) {
	fmt.Println("insert")
	var err error
	msg := fmt.Sprintf("insert,%s,%d,%d", name, faceID, cost)
	_, err = m.unix.Write([]byte(msg))
	if err != nil {
		fmt.Println("Write data failed:", err.Error())
	}
}

func (m *MgmtConn) SetCapacity(cap int) {
	var err error
	msg := fmt.Sprintf("set,%d", cap)
	_, err = m.unix.Write([]byte(msg))
	if err != nil {
		fmt.Println("Write data failed:", err.Error())
	}
}

func (m *MgmtConn) RunRead() {
	for {
		received := make([]byte, 1024)
		_, err := m.unix.Read(received)
		if err != nil {
			if err.Error() == "EOF" {
				fmt.Println("stopped reading")
				break
			}
		}
		command := bytes.Trim(received, "\x00")
		faceID, _ := strconv.Atoi(string(command))
		m.Table.CleanUpFace(uint64(faceID))
	}
}
