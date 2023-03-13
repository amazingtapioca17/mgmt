package mgmtconn

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/amazingtapioca17/mgmt/ribinterface"
	"github.com/named-data/YaNFD/ndn"
	"github.com/named-data/YaNFD/ndn/mgmt"
)

type AckConn struct {
	port   string
	socket string
	unix   net.Conn
	Table  ribinterface.RibInt
	queue  chan chan Message
}
type Message struct {
	Command         string                 `json:"command"`
	Name            string                 `json:"name"`
	ParamName       string                 `json:"paramname"`
	FaceID          uint64                 `json:"faceid"`
	Cost            uint64                 `json:"cost"`
	Strategy        string                 `json:"strategy"`
	Capacity        int                    `json:"capacity"`
	Versions        []uint64               `json:"versions"`
	Dataset         []byte                 `json:"dataset"`
	Valid           bool                   `json:"valid"`
	ControlParams   mgmt.ControlParameters `json:"controlparams"`
	ControlResponse mgmt.ControlResponse   `json:"controlresponse"`
	ErrorCode       int                    `json:"errorcode"`
	ErrorMessage    string                 `json:"errormessage"`
	ParamsValid     bool                   `json:"paramsvalid"`
	FaceQueryFilter mgmt.FaceQueryFilter   `json:"facequeryfilter"`
}

var AcksConn AckConn

func (a *AckConn) RunReceive() {
	for {
		message := make([]byte, 8800)
		readSize, err := a.unix.Read(message)
		message = message[:readSize]
		if err != nil {
			fmt.Println(err)
		}
		msg := a.ParseResponse(message)
		if msg.Command == "" {
			// empty command means it is a response
			// meaningful command means it is clean up face, and we need to continue to preserve ordered messages
			// does not work if it is not in a new goroutine, probably because a.Table.CleanupFace calls other send messages, would
			newRequest := <-a.queue
			newRequest <- msg
		} else {
			go a.Table.CleanUpFace(msg.FaceID)
		}
	}
}

func (a *AckConn) MakeMgmtConn(socket string) {
	a.socket = socket
	var err error
	a.unix, err = net.Dial("unixpacket", a.socket)
	fmt.Println("ack connected")
	if err != nil {
		fmt.Println("Error dialing socket:", err)
	}
	a.queue = make(chan chan Message)
}
func (a *AckConn) Conn() net.Conn {
	return a.unix
}

func (a *AckConn) GetAllFIBEntries() []byte {
	command := Message{
		Command: "list",
	}
	msg := a.SendCommand(command)
	return msg.Dataset
}
func (a *AckConn) ForwarderStatus() []byte {
	command := Message{
		Command: "forwarderstatus",
	}
	msg := a.SendCommand(command)
	return msg.Dataset
}

func (a *AckConn) Channels() []byte {
	command := Message{
		Command: "channels",
	}
	msg := a.SendCommand(command)
	return msg.Dataset
}

func (a *AckConn) ListFace() []byte {
	command := Message{
		Command: "listface",
	}
	msg := a.SendCommand(command)
	return msg.Dataset
}
func (a *AckConn) CreateFace(params mgmt.ControlParameters) mgmt.ControlResponse {
	command := Message{
		Command:       "createface",
		ControlParams: params,
	}
	msg := a.SendCommand(command)
	return msg.ControlResponse
}
func (a *AckConn) UpdateFace(params mgmt.ControlParameters, faceID uint64) mgmt.ControlResponse {
	command := Message{
		Command:       "updateface",
		ControlParams: params,
		FaceID:        faceID,
	}
	msg := a.SendCommand(command)
	return msg.ControlResponse
}
func (a *AckConn) DestroyFace(faceID uint64) bool {
	command := Message{
		Command: "destroyface",
		FaceID:  faceID,
	}
	msg := a.SendCommand(command)
	return msg.Valid
}

func (a *AckConn) Query(filter mgmt.FaceQueryFilter) []byte {
	command := Message{
		Command:         "query",
		FaceQueryFilter: filter,
	}
	msg := a.SendCommand(command)
	return msg.Dataset
}

func (a *AckConn) CsInfo() []byte {
	command := Message{
		Command: "info",
	}
	msg := a.SendCommand(command)
	return msg.Dataset
}

func (a *AckConn) Versions(strategy string) ([]uint64, bool) {
	command := Message{
		Command:  "versions",
		Strategy: strategy,
	}
	msg := a.SendCommand(command)
	return msg.Versions, msg.Valid
}

func (a *AckConn) ListStrategy() []byte {
	command := Message{
		Command: "liststrategy",
	}
	msg := a.SendCommand(command)
	return msg.Dataset
}

func (a *AckConn) GetFaceId(faceID uint64) bool {
	command := Message{
		Command: "faceid",
		FaceID:  faceID,
	}
	msg := a.SendCommand(command)
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

func (a *AckConn) ClearNextHops(name *ndn.Name) {
	msg := Message{
		Command: "clear",
		Name:    name.String(),
	}
	a.SendCommand(msg)
}

func (a *AckConn) RemoveNextHop(name *ndn.Name, faceID uint64) {
	msg := Message{
		Command: "remove",
		Name:    name.String(),
		FaceID:  faceID,
	}
	a.SendCommand(msg)
}

func (a *AckConn) InsertNextHop(name *ndn.Name, faceID uint64, cost uint64) {
	msg := Message{
		Command: "insert",
		Name:    name.String(),
		FaceID:  faceID,
		Cost:    cost,
	}
	a.SendCommand(msg)

}

func (a *AckConn) SetStrategy(paramName *ndn.Name, strategy *ndn.Name) {
	msg := Message{
		Command:   "setstrategy",
		ParamName: paramName.String(),
		Strategy:  strategy.String(),
	}
	a.SendCommand(msg)

}

func (a *AckConn) UnsetStrategy(paramName *ndn.Name) {
	msg := Message{
		Command:   "unsetstrategy",
		ParamName: paramName.String(),
	}
	a.SendCommand(msg)
}
func (a *AckConn) SetCapacity(cap int) {
	msg := Message{
		Command:  "set",
		Capacity: cap,
	}
	a.SendCommand(msg)

}

func (a *AckConn) SendCommand(command Message) Message {
	b, err := json.Marshal(command)
	if err != nil {
		fmt.Println("error:", err)
	}
	_, err = a.unix.Write(b)
	if err != nil {
		fmt.Println("Write data failed:", err.Error())
	}
	response := make(chan Message)
	a.queue <- response
	received := <-response
	return received
}
