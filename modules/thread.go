/* YaNFD - Yet another NDN Forwarding Daemon
 *
 * Copyright (C) 2020-2022 Eric Newberry.
 *
 * This file is licensed under the terms of the MIT License, as found in LICENSE.md.
 */

package modules

import (
	"fmt"
	"net"

	temp "github.com/amazingtapioca17/mgmt/transport"
	"github.com/named-data/YaNFD/core"
	"github.com/named-data/YaNFD/ndn"
	"github.com/named-data/YaNFD/ndn/mgmt"
	"github.com/named-data/YaNFD/ndn/tlv"
)

// Thread Represents the management thread
type Thread struct {
	transport      *temp.FakeTransport
	port           string
	localPrefix    *ndn.Name
	nonLocalPrefix *ndn.Name
	modules        map[string]Module
}

func (m *Thread) ClearNextHop(name *ndn.Name) string {
	udpServer, err := net.ResolveUDPAddr("udp", m.port)

	if err != nil {
		fmt.Println("ResolveUDPAddr failed:", err.Error())
	}

	conn, err := net.DialUDP("udp", nil, udpServer)
	if err != nil {
		fmt.Println("Listen failed:", err.Error())
	}

	//close the connection
	defer conn.Close()
	msg := fmt.Sprintf("clear,%s", name)
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

func (m *Thread) RemoveNextHop(name *ndn.Name, faceID uint64) string {
	udpServer, err := net.ResolveUDPAddr("udp", m.port)

	if err != nil {
		fmt.Println("ResolveUDPAddr failed:", err.Error())
	}

	conn, err := net.DialUDP("udp", nil, udpServer)
	if err != nil {
		fmt.Println("Listen failed:", err.Error())
	}

	//close the connection
	defer conn.Close()
	msg := fmt.Sprintf("insert,%s,%d", name, faceID)
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

func (m *Thread) InsertNextHop(name *ndn.Name, faceID uint64, cost uint64) string {
	udpServer, err := net.ResolveUDPAddr("udp", m.port)

	if err != nil {
		fmt.Println("ResolveUDPAddr failed:", err.Error())
	}

	conn, err := net.DialUDP("udp", nil, udpServer)
	if err != nil {
		fmt.Println("Listen failed:", err.Error())
	}

	//close the connection
	defer conn.Close()
	msg := fmt.Sprintf("insert,%s,%d,%d", name, faceID, cost)
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

// MakeMgmtThread creates a new management thread.
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
	m.port = ":1080"
	m.modules = make(map[string]Module)
	m.registerModule("cs", new(ContentStoreModule))
	m.registerModule("faces", new(FaceModule))
	m.registerModule("fib", new(FIBModule))
	m.registerModule("rib", new(RIBModule))
	m.registerModule("status", new(ForwarderStatusModule))
	m.registerModule("strategy-choice", new(StrategyChoiceModule))
	return m
}

func (m *Thread) String() string {
	return "Management"
}

func (m *Thread) registerModule(name string, module Module) {
	m.modules[name] = module
	module.registerManager(m)
}

func (m *Thread) prefixLength() int {
	return m.localPrefix.Size()
}

func (m *Thread) sendResponse(response *mgmt.ControlResponse, interest *ndn.Interest, pitToken []byte, inFace uint64) {
	encodedResponse, err := response.Encode()
	if err != nil {
		core.LogWarn(m, "Unable to send ControlResponse for ", interest.Name(), ": ", err)
		return
	}
	encodedWire, err := encodedResponse.Wire()
	if err != nil {
		core.LogWarn(m, "Unable to send ControlResponse for ", interest.Name(), ": ", err)
		return
	}
	data := ndn.NewData(interest.Name(), encodedWire)

	encodedData, err := data.Encode()
	if err != nil {
		core.LogWarn(m, "Unable to send ControlResponse for ", interest.Name(), ": ", err)
		return
	}

	m.transport.Send(encodedData, pitToken, &inFace)
}

// Run management thread
func (m *Thread) Run() {
	fmt.Println("running")
	m.transport = temp.MakeFakeTransport()
	go m.transport.RunReceive()
	// Create and register Internal transport
	for {
		block, pitToken, inFace := m.transport.Receive()
		if block == nil {
			// Indicates that internal face has quit, which means it's time for us to quit
			core.LogInfo(m, "Face quit, so management quitting")
			break
		}
		core.LogTrace(m, "Received block on face, IncomingFaceID=", inFace)

		// We only expect Interests, so drop Data packets
		if block.Type() != tlv.Interest {
			core.LogWarn(m, "Dropping received non-Interest packet of type ", block.Type())
			continue
		}
		interest, err := ndn.DecodeInterest(block)
		if err != nil {
			core.LogWarn(m, "Unable to decode received Interest: ", err, " - DROP")
			continue
		}

		// Ensure Interest name matches expectations
		if interest.Name().Size() < m.localPrefix.Size()+2 { // Module + Verb
			core.LogInfo(m, "Control command name ", interest.Name(), " has unexpected number of components - DROP")
			continue
		}
		if !m.localPrefix.PrefixOf(interest.Name()) && !m.nonLocalPrefix.PrefixOf(interest.Name()) {
			core.LogInfo(m, "Control command name ", interest.Name(), " has unexpected prefix - DROP")
			continue
		}

		core.LogTrace(m, "Received management Interest ", interest.Name())

		// Dispatch interest based on name
		moduleName := interest.Name().At(m.localPrefix.Size()).String()
		if module, ok := m.modules[moduleName]; ok {
			module.handleIncomingInterest(interest, pitToken, inFace)
		} else {
			core.LogWarn(m, "Received management Interest for unknown module ", moduleName)
			response := mgmt.MakeControlResponse(501, "Unknown module", nil)
			m.sendResponse(response, interest, pitToken, inFace)
		}
	}
}
