package main

import (
	"github.com/pion/webrtc/v2"
)

var (
	configRTC = webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			// {
			// 	URLs: []string{
			// 		"stun:stun.l.google.com:19302",
			// 	},
			// },
			{
				URLs: []string{
					"turn:turn.stublab.io:3478",
				},
				Username:       string("turnserver"),
				Credential:     "*StubLab-02-2019*",
				CredentialType: webrtc.ICECredentialTypePassword,
			},
			{
				URLs: []string{
					"stun:turn.stublab.io:3478",
				},
			},
		},
		// ICETransportPolicy: webrtc.ICETransportPolicyRelay,
	}
	defaultHost = "127.0.0.1"
	defaultPort = 22
)
