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

func (m *MgmtConn) MakeMgmtConn() {
	m.socket = "/tmp/mgmt.sock"
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
	// udpServer, err := net.ResolveUDPAddr("udp", m.Port)

	// if err != nil {
	// 	fmt.Println("ResolveUDPAddr failed:", err.Error())
	// }

	// conn, err := net.DialUDP("udp", nil, udpServer)
	// if err != nil {
	// 	fmt.Println("Listen failed:", err.Error())
	// }
	// Create a new Unix packet dialer
	// conn, err := net.Dial("unixpacket", m.socket)
	// if err != nil {
	// 	fmt.Println("Error dialing socket:", err)
	// }
	// defer conn.Close()
	var err error
	msg := fmt.Sprintf("clear,%s", name)
	_, err = m.unix.Write([]byte(msg))
	if err != nil {
		fmt.Println("Write data failed:", err.Error())
	}

	// buffer to get data
}

func (m *MgmtConn) RemoveNextHop(name *ndn.Name, faceID uint64) {
	fmt.Println("remove")
	// udpServer, err := net.ResolveUDPAddr("udp", m.Port)

	// if err != nil {
	// 	fmt.Println("ResolveUDPAddr failed:", err.Error())
	// }

	// conn, err := net.DialUDP("udp", nil, udpServer)
	// if err != nil {
	// 	fmt.Println("Listen failed:", err.Error())
	// }

	// Create a new Unix packet dialer
	// conn, err := net.Dial("unixpacket", m.socket)
	// if err != nil {
	// 	fmt.Println("Error dialing socket:", err)
	// }
	// //close the connection
	// defer conn.Close()
	var err error
	msg := fmt.Sprintf("insert,%s,%d", name, faceID)
	_, err = m.unix.Write([]byte(msg))
	if err != nil {
		fmt.Println("Write data failed:", err.Error())
	}

	// buffer to get data
	// received := make([]byte, 1024)
	// _, err = conn.Read(received)
	// if err != nil {
	// 	fmt.Println("Read data failed:", err.Error())
	// }
	// return string(received)
}

func (m *MgmtConn) InsertNextHop(name *ndn.Name, faceID uint64, cost uint64) {
	fmt.Println("insert")
	// udpServer, err := net.ResolveUDPAddr("udp", m.Port)

	// if err != nil {
	// 	fmt.Println("ResolveUDPAddr failed:", err.Error())
	// }

	// conn, err := net.DialUDP("udp", nil, udpServer)
	// if err != nil {
	// 	fmt.Println("Listen failed:", err.Error())
	// }

	// Create a new Unix packet dialer
	// conn, err := net.Dial("unixpacket", m.socket)
	// if err != nil {
	// 	fmt.Println("Error dialing socket:", err)
	// }
	// //close the connection
	// defer conn.Close()
	var err error
	msg := fmt.Sprintf("insert,%s,%d,%d", name, faceID, cost)
	_, err = m.unix.Write([]byte(msg))
	if err != nil {
		fmt.Println("Write data failed:", err.Error())
	}

	// // buffer to get data
	// received := make([]byte, 1024)
	// _, err = conn.Read(received)
	// if err != nil {
	// 	fmt.Println("Read data failed:", err.Error())
	// }
	// return string(received)
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
