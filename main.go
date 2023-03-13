package main

import (
	"fmt"

	"github.com/amazingtapioca17/mgmt/mgmtconn"
	"github.com/amazingtapioca17/mgmt/modules"

	customrib "github.com/amazingtapioca17/mgmt/table"
	"github.com/named-data/YaNFD/ndn"
	"github.com/named-data/YaNFD/ndn/lpv2"
	"github.com/named-data/YaNFD/ndn/mgmt"
	"github.com/named-data/YaNFD/ndn/tlv"
)

var partialMessageStore map[uint64][][]byte

func main() {
	//go mgmtConn()
	// mgmtconn.Conn.Port = ":1080"
	// mgmtconn.Conn.Socket = "/tmp/fib.sock"
	go input1()
	mgmtconn.AcksConn.MakeMgmtConn("/tmp/ackmgmt.sock")
	mgmtconn.AcksConn.Table = &customrib.Rib
	go mgmtconn.AcksConn.RunReceive()

	manager := modules.MakeMgmtThread()
	manager.Run()

}

func input1() {
	var i string
	fmt.Scan(&i)
	//test, _ := ndn.NameFromString("/ppp/poem")
	//mgmtconn.AcksConn.RemoveNextHop(test, 4)
	fmt.Println(mgmtconn.AcksConn.ForwarderStatus())
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
