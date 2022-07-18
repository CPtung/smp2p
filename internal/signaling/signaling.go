package signaling

import (
	"github.com/pion/webrtc/v3"
)

type DescInfo struct {
	Name string                    `json:"name"`
	Sdp  webrtc.SessionDescription `json:"sdp"`
}

type OnDescReceived func(desc []byte)

type OnCandidateReceived func(candidate []byte)

type Signal interface {
	Connect() error
	Disconnect()
	SendICESdp(string, []byte) error
	SendICECandidate(string, string) error
	AddICESdpListener(OnDescReceived)
	AddICECandidateListener(OnCandidateReceived)
}
