package p2p

import (
	"fmt"

	"github.com/CPtung/smp2p/pkg/model"
	"github.com/pion/webrtc/v3"
)

const (
	SshSessionType = "ssh"
	ScpSessionType = "scp"
	WebSessionType = "web"
)

type SessionDesc struct {
	Name        string                    `json:"name"`
	Type        string                    `json:"type" validate:"oneof=ssh scp web"`
	ForwardAddr string                    `json:"forwardAdd"`
	Sdp         webrtc.SessionDescription `json:"sdp"`
}

type OnDescReceived func(desc []byte)

type OnCandidateReceived func(candidate []byte)

type Signal interface {
	ForcePublish(msg model.Message, compress bool) error
	ForceSubscribe(string, func(string, []byte))
}

var (
	P2PSignalSdpF       = "stunnelv1.0/%s/p2p/sdp"
	P2PSignalCandidateF = "stunnelv1.0/%s/p2p/candidate"
)

func GetP2PSdpTopic(sessionID string) string {
	return fmt.Sprintf(P2PSignalSdpF, sessionID)
}

func GetP2PCandidateTopic(sessionID string) string {
	return fmt.Sprintf(P2PSignalCandidateF, sessionID)
}
