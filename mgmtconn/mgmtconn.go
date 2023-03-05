package mgmtconn

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"

	"github.com/amazingtapioca17/mgmt/ribinterface"
	"github.com/named-data/YaNFD/ndn"
)

type MgmtConn struct {
	port   string
	socket string
	unix   net.Conn
	Table  ribinterface.RibInt
}

type Message struct {
	Command   string   `json:"command"`
	Name      string   `json:"name"`
	ParamName string   `json:"paramname"`
	FaceID    uint64   `json:"faceid"`
	Cost      uint64   `json:"cost"`
	Strategy  string   `json:"strategy"`
	Capacity  int      `json:"capacity"`
	Versions  []uint64 `json:"versions"`
	Dataset   []byte   `json:"dataset"`
	Valid     bool     `json:"valid"`
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
	//var commands = []string{"clear", name.String()}
	msg := Message{
		Command: "clear",
		Name:    name.String(),
	}
	m.SendCommand(msg)
}

func (m *MgmtConn) RemoveNextHop(name *ndn.Name, faceID uint64) {
	//var commands = []string{"remove", name.String(), fmt.Sprintf("%d", faceID)}
	msg := Message{
		Command: "remove",
		Name:    name.String(),
		FaceID:  faceID,
	}
	m.SendCommand(msg)
}

func (m *MgmtConn) InsertNextHop(name *ndn.Name, faceID uint64, cost uint64) {
	//var commands = []string{"insert", name.String(), fmt.Sprintf("%d", faceID), fmt.Sprintf("%d", cost)}
	msg := Message{
		Command: "insert",
		Name:    name.String(),
		FaceID:  faceID,
		Cost:    cost,
	}
	m.SendCommand(msg)

}

func (m *MgmtConn) SetStrategy(paramName *ndn.Name, strategy *ndn.Name) {
	msg := Message{
		Command:   "setstrategy",
		ParamName: paramName.String(),
		Strategy:  strategy.String(),
	}
	m.SendCommand(msg)

}

func (m *MgmtConn) UnsetStrategy(paramName *ndn.Name) {
	msg := Message{
		Command:   "unsetstrategy",
		ParamName: paramName.String(),
	}
	m.SendCommand(msg)
}
func (m *MgmtConn) SetCapacity(cap int) {
	msg := Message{
		Command:  "set",
		Capacity: cap,
	}
	m.SendCommand(msg)

}

func (m *MgmtConn) SendCommand(command Message) {
	b, err := json.Marshal(command)
	if err != nil {
		fmt.Println("error:", err)
	}
	_, err = m.unix.Write(b)
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
		received = bytes.Trim(received, "\x00")
		var commands Message
		err = json.Unmarshal(received, &commands)
		if err != nil {
			fmt.Println("error:", err)
		}
		m.Table.CleanUpFace(commands.FaceID)
	}
}
