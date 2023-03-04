/* YaNFD - Yet another NDN Forwarding Daemon
 *
 * Copyright (C) 2020-2021 Eric Newberry.
 *
 * This file is licensed under the terms of the MIT License, as found in LICENSE.md.
 */

package modules

import (
	"github.com/amazingtapioca17/mgmt/mgmtconn"
	"github.com/named-data/YaNFD/core"
	"github.com/named-data/YaNFD/ndn"
	"github.com/named-data/YaNFD/ndn/mgmt"
)

// ForwarderStatusModule is the module that provide forwarder status information.
type ForwarderStatusModule struct {
	manager                   *Thread
	nextGeneralDatasetVersion uint64
}

func (f *ForwarderStatusModule) String() string {
	return "ForwarderStatusMgmt"
}

func (f *ForwarderStatusModule) registerManager(manager *Thread) {
	f.manager = manager
}

func (f *ForwarderStatusModule) getManager() *Thread {
	return f.manager
}

func (f *ForwarderStatusModule) handleIncomingInterest(interest *ndn.Interest, pitToken []byte, inFace uint64) {
	// Only allow from /localhost
	if !f.manager.localPrefix.PrefixOf(interest.Name()) {
		core.LogWarn(f, "Received forwarder status management Interest from non-local source - DROP")
		return
	}

	// Dispatch by verb
	verb := interest.Name().At(f.manager.prefixLength() + 1).String()
	switch verb {
	case "general":
		f.general(interest, pitToken, inFace)
	default:
		core.LogWarn(f, "Received Interest for non-existent verb '", verb, "'")
		response := mgmt.MakeControlResponse(501, "Unknown verb", nil)
		f.manager.sendResponse(response, interest, pitToken, inFace)
		return
	}
}

func (f *ForwarderStatusModule) general(interest *ndn.Interest, pitToken []byte, inFace uint64) {
	if interest.Name().Size() > f.manager.prefixLength()+2 {
		// Ignore because contains version and/or segment components
		return
	}

	// Generate new dataset
	dataset := mgmtconn.AcksConn.ForwarderStatus()

	name, _ := ndn.NameFromString(f.manager.localPrefix.String() + "/status/general")
	segments := mgmt.MakeStatusDataset(name, f.nextGeneralDatasetVersion, dataset)
	for _, segment := range segments {
		encoded, err := segment.Encode()
		if err != nil {
			core.LogError(f, "Unable to encode forwarder status dataset: ", err)
			return
		}
		f.manager.transport.Send(encoded, pitToken, nil)
	}

	core.LogTrace(f, "Published forwarder status dataset version=", f.nextGeneralDatasetVersion, ", containing ", len(segments), " segments")
	f.nextGeneralDatasetVersion++
}
