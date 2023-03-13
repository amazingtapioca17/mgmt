/* YaNFD - Yet another NDN Forwarding Daemon
 *
 * Copyright (C) 2020-2022 Eric Newberry.
 *
 * This file is licensed under the terms of the MIT License, as found in LICENSE.md.
 */

package modules

import (
	"net"
	"time"

	"github.com/amazingtapioca17/mgmt/mgmtconn"
	"github.com/named-data/YaNFD/core"
	"github.com/named-data/YaNFD/face"
	"github.com/named-data/YaNFD/ndn"
	"github.com/named-data/YaNFD/ndn/mgmt"
	"github.com/named-data/YaNFD/ndn/tlv"
)

// FaceModule is the module that handles Face Management.
type FaceModule struct {
	manager                   *Thread
	nextFaceDatasetVersion    uint64
	nextChannelDatasetVersion uint64
}

func (f *FaceModule) String() string {
	return "FaceMgmt"
}

func (f *FaceModule) registerManager(manager *Thread) {
	f.manager = manager
	face.FaceEventSendFunc = f.sendFaceEventNotification
}

func (f *FaceModule) getManager() *Thread {
	return f.manager
}

func (f *FaceModule) handleIncomingInterest(interest *ndn.Interest, pitToken []byte, inFace uint64) {
	// Only allow from /localhost
	if !f.manager.localPrefix.PrefixOf(interest.Name()) {
		core.LogWarn(f, "Received face management Interest from non-local source - DROP")
		return
	}

	// Dispatch by verb
	verb := interest.Name().At(f.manager.prefixLength() + 1).String()
	switch verb {
	case "create":
		f.create(interest, pitToken, inFace)
	case "update":
		f.update(interest, pitToken, inFace)
	case "destroy":
		f.destroy(interest, pitToken, inFace)
	case "list":
		f.list(interest, pitToken, inFace)
	case "query":
		f.query(interest, pitToken, inFace)
	case "channels":
		f.channels(interest, pitToken, inFace)
	case "events":
		f.events(interest, pitToken, inFace)
	default:
		core.LogWarn(f, "Received Interest for non-existent verb '", verb, "'")
		response := mgmt.MakeControlResponse(501, "Unknown verb", nil)
		f.manager.sendResponse(response, interest, pitToken, inFace)
		return
	}
}

func (f *FaceModule) create(interest *ndn.Interest, pitToken []byte, inFace uint64) {
	var response *mgmt.ControlResponse

	if interest.Name().Size() < f.manager.prefixLength()+3 {
		// Name not long enough to contain ControlParameters
		core.LogWarn(f, "Missing ControlParameters in ", interest.Name())
		response = mgmt.MakeControlResponse(400, "ControlParameters is incorrect", nil)
		f.manager.sendResponse(response, interest, pitToken, inFace)
		return
	}

	params := decodeControlParameters(f, interest)
	if params == nil {
		response = mgmt.MakeControlResponse(400, "ControlParameters is incorrect", nil)
		f.manager.sendResponse(response, interest, pitToken, inFace)
		return
	}

	if params.URI == nil {
		core.LogWarn(f, "Missing URI in ControlParameters for ", interest.Name())
		response = mgmt.MakeControlResponse(400, "ControlParameters is incorrect", nil)
		f.manager.sendResponse(response, interest, pitToken, inFace)
		return
	}

	if params.URI.Canonize() != nil {
		core.LogWarn(f, "Cannot canonize remote URI in ControlParameters for ", interest.Name())
		response = mgmt.MakeControlResponse(406, "URI could not be canonized", nil)
		f.manager.sendResponse(response, interest, pitToken, inFace)
		return
	}

	if (params.Flags != nil && params.Mask == nil) || (params.Flags == nil && params.Mask != nil) {
		core.LogWarn(f, "Flags and Mask fields either both be present or both be not present")
		response = mgmt.MakeControlResponse(409, "Incomplete Flags/Mask combination", nil)
		f.manager.sendResponse(response, interest, pitToken, inFace)
		return
	}

	// Ensure does not conflict with existing face
	forwarderResponse := mgmtconn.AcksConn.CreateFace(*params)
	response = &forwarderResponse
	f.manager.sendResponse(response, interest, pitToken, inFace)
}

func (f *FaceModule) update(interest *ndn.Interest, pitToken []byte, inFace uint64) {
	var response *mgmt.ControlResponse

	if interest.Name().Size() < f.manager.prefixLength()+3 {
		// Name not long enough to contain ControlParameters
		core.LogWarn(f, "Missing ControlParameters in ", interest.Name())
		response = mgmt.MakeControlResponse(400, "ControlParameters is incorrect", nil)
		f.manager.sendResponse(response, interest, pitToken, inFace)
		return
	}

	params := decodeControlParameters(f, interest)
	if params == nil {
		response = mgmt.MakeControlResponse(400, "ControlParameters is incorrect", nil)
		f.manager.sendResponse(response, interest, pitToken, inFace)
		return
	}

	faceID := inFace
	if params.FaceID != nil && *params.FaceID != 0 {
		faceID = *params.FaceID
	}

	// Validate parameters
	forwarderResponse := mgmtconn.AcksConn.UpdateFace(*params, faceID)
	response = &forwarderResponse
	f.manager.sendResponse(response, interest, pitToken, inFace)
}

func (f *FaceModule) destroy(interest *ndn.Interest, pitToken []byte, inFace uint64) {
	var response *mgmt.ControlResponse

	if interest.Name().Size() < f.manager.prefixLength()+3 {
		// Name not long enough to contain ControlParameters
		core.LogWarn(f, "Missing ControlParameters in ", interest.Name())
		response = mgmt.MakeControlResponse(400, "ControlParameters is incorrect", nil)
		f.manager.sendResponse(response, interest, pitToken, inFace)
		return
	}

	params := decodeControlParameters(f, interest)
	if params == nil {
		response = mgmt.MakeControlResponse(400, "ControlParameters is incorrect", nil)
		f.manager.sendResponse(response, interest, pitToken, inFace)
		return
	}

	if params.FaceID == nil {
		core.LogWarn(f, "Missing FaceId in ControlParameters for ", interest.Name())
		response = mgmt.MakeControlResponse(400, "ControlParameters is incorrect", nil)
		f.manager.sendResponse(response, interest, pitToken, inFace)
		return
	}

	mgmtconn.AcksConn.DestroyFace(*params.FaceID)

	responseParamsWire, err := params.Encode()
	if err != nil {
		core.LogError(f, "Unable to encode response parameters: ", err)
		response = mgmt.MakeControlResponse(500, "Internal error", nil)
	} else {
		response = mgmt.MakeControlResponse(200, "OK", responseParamsWire)
	}
	forwarderResponse := mgmtconn.AcksConn.CreateFace(*params)
	response = &forwarderResponse
	f.manager.sendResponse(response, interest, pitToken, inFace)
}

func (f *FaceModule) list(interest *ndn.Interest, pitToken []byte, inFace uint64) {
	if interest.Name().Size() > f.manager.prefixLength()+2 {
		// Ignore because contains version and/or segment components
		return
	}

	// Generate new dataset
	dataset := mgmtconn.AcksConn.ListFace()
	name, _ := ndn.NameFromString(f.manager.localPrefix.String() + "/faces/list")
	segments := mgmt.MakeStatusDataset(name, f.nextFaceDatasetVersion, dataset)
	for _, segment := range segments {
		encoded, err := segment.Encode()
		if err != nil {
			core.LogError(f, "Unable to encode face status dataset: ", err)
			return
		}
		f.manager.transport.Send(encoded, pitToken, nil)
	}

	core.LogTrace(f, "Published face dataset version=", f.nextFaceDatasetVersion, ", containing ", len(segments), " segments")
	f.nextFaceDatasetVersion++
}

func (f *FaceModule) query(interest *ndn.Interest, pitToken []byte, inFace uint64) {
	if interest.Name().Size() < f.manager.prefixLength()+3 {
		// Name not long enough to contain FaceQueryFilter
		core.LogWarn(f, "Missing FaceQueryFilter in ", interest.Name())
		return
	}

	filter, err := mgmt.DecodeFaceQueryFilterFromEncoded(interest.Name().At(f.manager.prefixLength() + 2).Value())
	if err != nil {
		return
	}

	dataset := mgmtconn.AcksConn.Query(*filter)

	segments := mgmt.MakeStatusDataset(interest.Name(), f.nextFaceDatasetVersion, dataset)
	for _, segment := range segments {
		encoded, err := segment.Encode()
		if err != nil {
			core.LogError(f, "Unable to encode face query dataset: ", err)
			return
		}
		f.manager.transport.Send(encoded, pitToken, nil)
	}

	core.LogTrace(f, "Published face query dataset version=", f.nextFaceDatasetVersion, ", containing ", len(segments), " segments")
	f.nextFaceDatasetVersion++
}

func (f *FaceModule) createDataset(selectedFace face.LinkService) []byte {
	faceDataset := mgmt.MakeFaceStatus()
	faceDataset.FaceID = uint64(selectedFace.FaceID())
	faceDataset.URI = selectedFace.RemoteURI()
	faceDataset.LocalURI = selectedFace.LocalURI()
	if selectedFace.ExpirationPeriod() != 0 {
		faceDataset.ExpirationPeriod = new(uint64)
		*faceDataset.ExpirationPeriod = uint64(selectedFace.ExpirationPeriod().Milliseconds())
	}
	faceDataset.FaceScope = uint64(selectedFace.Scope())
	faceDataset.FacePersistency = uint64(selectedFace.Persistency())
	faceDataset.LinkType = uint64(selectedFace.LinkType())
	faceDataset.MTU = new(uint64)
	*faceDataset.MTU = uint64(selectedFace.MTU())
	faceDataset.NInInterests = selectedFace.NInInterests()
	faceDataset.NInData = selectedFace.NInData()
	faceDataset.NInNacks = 0
	faceDataset.NOutInterests = selectedFace.NOutInterests()
	faceDataset.NOutData = selectedFace.NOutData()
	faceDataset.NOutNacks = 0
	faceDataset.NInBytes = selectedFace.NInBytes()
	faceDataset.NOutBytes = selectedFace.NOutBytes()
	linkService, ok := selectedFace.(*face.NDNLPLinkService)
	if ok {
		options := linkService.Options()

		faceDataset.BaseCongestionMarkingInterval = new(uint64)
		*faceDataset.BaseCongestionMarkingInterval = uint64(options.BaseCongestionMarkingInterval.Nanoseconds())
		faceDataset.DefaultCongestionThreshold = new(uint64)
		*faceDataset.DefaultCongestionThreshold = options.DefaultCongestionThresholdBytes
		faceDataset.Flags = options.Flags()
		if options.IsConsumerControlledForwardingEnabled {
			// This one will only be enabled if the other two local fields are enabled (and vice versa)
			faceDataset.Flags |= face.FaceFlagLocalFields
		}
		if options.IsReliabilityEnabled {
			faceDataset.Flags |= face.FaceFlagLpReliabilityEnabled
		}
		if options.IsCongestionMarkingEnabled {
			faceDataset.Flags |= face.FaceFlagCongestionMarking
		}
	}

	faceDatasetEncoded, err := faceDataset.Encode()
	if err != nil {
		core.LogError(f, "Cannot encode FaceStatus for FaceID=", selectedFace.FaceID(), ": ", err)
		return []byte{}
	}
	faceDatasetWire, err := faceDatasetEncoded.Wire()
	if err != nil {
		core.LogError(f, "Cannot encode FaceStatus for FaceID=", selectedFace.FaceID(), ": ", err)
		return []byte{}
	}
	return faceDatasetWire
}

func (f *FaceModule) channels(interest *ndn.Interest, pitToken []byte, inFace uint64) {
	if interest.Name().Size() < f.manager.prefixLength()+2 {
		core.LogWarn(f, "Channel dataset Interest too short: ", interest.Name())
		return
	}

	dataset := make([]byte, 0)
	// UDP channel
	ifaces, err := net.Interfaces()
	if err != nil {
		core.LogWarn(f, "Unable to access channel dataset: ", err)
		return
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			core.LogWarn(f, "Unable to access IP addresses for ", iface.Name, ": ", err)
			return
		}
		for _, addr := range addrs {
			ipAddr := addr.(*net.IPNet)

			ipVersion := 4
			path := ipAddr.IP.String()
			if ipAddr.IP.To4() == nil {
				ipVersion = 6
				path += "%" + iface.Name
			}

			if !addr.(*net.IPNet).IP.IsLoopback() {
				uri := ndn.MakeUDPFaceURI(ipVersion, path, face.UDPUnicastPort)
				channel := mgmt.MakeChannelStatus(uri)
				channelEncoded, err := channel.Encode()
				if err != nil {
					core.LogError(f, "Cannot encode ChannelStatus for Channel=", uri, ": ", err)
					continue
				}
				channelWire, err := channelEncoded.Wire()
				if err != nil {
					core.LogError(f, "Cannot encode ChannelStatus for Channel=", uri, ": ", err)
					continue
				}
				dataset = append(dataset, channelWire...)
			}
		}
	}

	// Unix channel
	uri := ndn.MakeUnixFaceURI(face.UnixSocketPath)
	channel := mgmt.MakeChannelStatus(uri)
	channelEncoded, err := channel.Encode()
	if err != nil {
		core.LogError(f, "Cannot encode ChannelStatus for Channel=", uri, ": ", err)
		return
	}
	channelWire, err := channelEncoded.Wire()
	if err != nil {
		core.LogError(f, "Cannot encode ChannelStatus for Channel=", uri, ": ", err)
		return
	}
	dataset = append(dataset, channelWire...)

	segments := mgmt.MakeStatusDataset(interest.Name(), f.nextChannelDatasetVersion, dataset)
	for _, segment := range segments {
		encoded, err := segment.Encode()
		if err != nil {
			core.LogError(f, "Unable to encode channel dataset: ", err)
			return
		}
		f.manager.transport.Send(encoded, pitToken, nil)
	}

	core.LogTrace(f, "Published channel dataset version=", f.nextChannelDatasetVersion, ", containing ", len(segments), " segments")
	f.nextChannelDatasetVersion++
}

func (f *FaceModule) fillFaceProperties(params *mgmt.ControlParameters, selectedFace face.LinkService) {
	params.FaceID = new(uint64)
	*params.FaceID = uint64(selectedFace.FaceID())
	params.URI = selectedFace.RemoteURI()
	params.LocalURI = selectedFace.LocalURI()
	params.FacePersistency = new(uint64)
	*params.FacePersistency = uint64(selectedFace.Persistency())
	params.MTU = new(uint64)
	*params.MTU = uint64(selectedFace.MTU())

	params.Flags = new(uint64)
	*params.Flags = 0
	linkService, ok := selectedFace.(*face.NDNLPLinkService)
	if ok {
		options := linkService.Options()

		params.BaseCongestionMarkingInterval = new(uint64)
		*params.BaseCongestionMarkingInterval = uint64(options.BaseCongestionMarkingInterval.Nanoseconds())
		params.DefaultCongestionThreshold = new(uint64)
		*params.DefaultCongestionThreshold = options.DefaultCongestionThresholdBytes
		*params.Flags = options.Flags()
	}
}

func (f *FaceModule) events(interest *ndn.Interest, pitToken []byte, inFace uint64) {
	var id uint64 = 0
	var err error

	if interest.Name().Size() < f.manager.prefixLength()+3 {
		// Name is a prefix, take the last one
		id = face.FaceEventLastId()
		if !interest.CanBePrefix() {
			core.LogInfo(f, "FaceEvent Interest with a prefix should set CanBePrefix=true: ", interest.Name())
			return
		}
	} else {
		seg, ok := interest.Name().At(f.manager.prefixLength() + 2).(*ndn.SequenceNumNameComponent)
		if !ok {
			core.LogInfo(f, "FaceEvent Interest with an illegible event ID: ", interest.Name())
			return
		}
		id, err = tlv.DecodeNNI(seg.Value())
		if err != nil {
			core.LogInfo(f, "FaceEvent Interest with an illegible event ID: ", interest.Name(), "err: ", err)
			return
		}
	}

	f.sendFaceEventNotification(id, pitToken)
}

func (f *FaceModule) sendFaceEventNotification(id uint64, pitToken []byte) {
	event := face.GetFaceEvent(id)
	if event == nil {
		return
	}

	eventBlock, err := event.Encode()
	if err != nil {
		core.LogError(f, "Cannot encode FaceEventNotification for EventID=", id, ": ", err)
		return
	}
	wire, err := eventBlock.Wire()
	if err != nil {
		core.LogError(f, "Cannot encode FaceEventNotification for EventID=", id, ": ", err)
		return
	}

	dataName, err := ndn.NameFromString("/localhost/nfd/faces/events")
	if err != nil {
		core.LogError(f, "Cannot encode FaceEventNotification name.")
		return
	}
	dataName = dataName.Append(ndn.NewSequenceNumNameComponent(id))
	data := ndn.NewData(dataName, wire)
	metaInfo := ndn.NewMetaInfo()
	metaInfo.SetFreshnessPeriod(1 * time.Millisecond)
	data.SetMetaInfo(metaInfo)

	encodedData, err := data.Encode()
	if err != nil {
		core.LogError(f, "Cannot encode FaceEventNotification data for EventID=", id, ": ", err)
		return
	}
	if f.manager.transport != nil {
		f.manager.transport.Send(encodedData, pitToken, nil)
	}
}
