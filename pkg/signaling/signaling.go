package signaling

import (
	"log"

	"github.com/pion/webrtc/v3"
)

type CandidateInfo struct {
	Source    string `json:"source"`
	Candidate string `json:"candidate"`
}

type SdpInfo struct {
	Source string `json:"source"`
	Sdp    string `json:"sdp"`
}

type SignalServer interface {
}

type Offer interface {
	AddCandidateListener()
	AddSdpListener()
	Close()
}

type Answer interface {
	SignalSdp(sdp []byte) error
	SignalCandidate(c *webrtc.ICECandidate) error
	Close()
}

type Desc struct {
	Name string
}

type Wrap struct {
	*webrtc.DataChannel
}

func (rtc *Wrap) Write(data []byte) (int, error) {
	err := rtc.DataChannel.Send(data)
	if err != nil {
		log.Printf("wrap write error: %s", err.Error())
	}
	return len(data), err
}
