package peer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/CPtung/smp2p/internal/signaling"
	"github.com/CPtung/smp2p/pkg/mqtt"
	"github.com/CPtung/smp2p/pkg/session"
	"github.com/pion/webrtc/v3"
)

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

type PeerConnection struct {
	name   string
	signal signaling.Signal
	pc     *webrtc.PeerConnection
}

func Init(name string) *PeerConnection {

	signal := mqtt.New(name)
	if err := signal.Connect(); err != nil {
		return nil
	}

	return &PeerConnection{
		name:   name,
		signal: signal,
	}
}

func (p *PeerConnection) Create() (err error) {
	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}
	// Create a new PeerConnection
	p.pc, err = webrtc.NewPeerConnection(config)
	return err
}

func (p *PeerConnection) CreateAnswerDesc(offerDesc []byte) (string, []byte, error) {
	offer := signaling.DescInfo{}
	if err := json.Unmarshal(offerDesc, &offer); err != nil {
		return "", nil, err
	}
	if sdpErr := p.pc.SetRemoteDescription(offer.Sdp); sdpErr != nil {
		return "", nil, sdpErr
	}
	// Set ICE Candidate handler. As soon as a PeerConnection has gathered a candidate
	// send it to the other peer
	p.pc.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i == nil {
			return
		}
		log.Println("on ice candidate....")
		if err := p.signal.SendICECandidate(offer.Name, i.ToJSON().Candidate); err != nil {
			log.Println(err.Error())
		}
	})

	p.signal.AddICECandidateListener(func(candidate []byte) {
		if candidateErr := p.pc.AddICECandidate(
			webrtc.ICECandidateInit{Candidate: string(candidate)}); candidateErr != nil {
			log.Printf("Peer AddICECandidate error: %s", candidateErr.Error())
		}
	})

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	p.pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s (answerer)\n", s.String())

		if s == webrtc.PeerConnectionStateFailed {
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			fmt.Println("Peer Connection has gone to failed exiting")
		} else if s == webrtc.PeerConnectionStateClosed {
			fmt.Println("Peer Connection has closed")
		}
	})

	answer, err := p.pc.CreateAnswer(nil)
	if err != nil {
		return "", nil, err
	}

	if err := p.pc.SetLocalDescription(answer); err != nil {
		return "", nil, err
	}

	desc := signaling.DescInfo{
		Name: p.name,
		Sdp:  answer,
	}
	// return answer's sdp
	data, err := json.Marshal(&desc)
	return offer.Name, data, err
}

func (p *PeerConnection) BindOfferDataChannel(s session.Session) error {

	if err := s.Create(); err != nil {
		return err
	}

	// Create a datachannel with label 'data'
	dc, err := p.pc.CreateDataChannel("data", nil)
	if err != nil {
		return err
	}

	dc.OnOpen(func() {
		s.OnOpen(&Wrap{DataChannel: dc})
		p.pc.Close()
		os.Exit(0)
	})

	// Register the OnMessage to handle incoming messages
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		s.OnMessage(msg.Data)
	})

	dc.OnClose(s.OnClose)

	return nil
}

func (p *PeerConnection) CreateAnswerDataService(s session.Session) error {

	if err := s.Create(); err != nil {
		return err
	}

	p.pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		// Register channel opening handling
		dc.OnOpen(func() {
			s.OnOpen(&Wrap{DataChannel: dc})
		})

		// Register the OnMessage to handle incoming messages
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			s.OnMessage(msg.Data)
		})

		dc.OnClose(func() {
			s.OnClose()
			p.pc.Close()
		})
	})

	return nil
}

func (p *PeerConnection) CreateOfferDesc(remote string) ([]string, []byte, error) {
	pendingCandidates := make([]string, 0)
	// When an ICE candidate is available send to the other Pion instance
	// the other Pion instance will add this candidate by calling AddICECandidate
	p.pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			log.Println("ignore empty candidate...")
			return
		}
		desc := p.pc.RemoteDescription()
		if desc == nil {
			pendingCandidates = append(pendingCandidates, c.ToJSON().Candidate)
		} else {
			if err := p.signal.SendICECandidate(remote, c.ToJSON().Candidate); err != nil {
				log.Println(err.Error())
			}
		}
	})

	p.signal.AddICECandidateListener(func(candidate []byte) {
		if candidateErr := p.pc.AddICECandidate(
			webrtc.ICECandidateInit{Candidate: string(candidate)}); candidateErr != nil {
			log.Printf("Peer AddICECandidate error: %s", candidateErr.Error())
		}
	})

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	p.pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", s.String())

		if s == webrtc.PeerConnectionStateFailed {
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			fmt.Println("Peer Connection has gone to failed exiting")
			os.Exit(0)
		}
	})

	// Create an offer to send to the other process
	offer, err := p.pc.CreateOffer(nil)
	if err != nil {
		return pendingCandidates, nil, err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	// Note: this will start the gathering of ICE candidates
	if err = p.pc.SetLocalDescription(offer); err != nil {
		return pendingCandidates, nil, err
	}

	// Send our offer to the HTTP signaling server listening in the other process
	desc := signaling.DescInfo{
		Name: p.name,
		Sdp:  offer,
	}
	// return answer's sdp
	data, err := json.Marshal(&desc)
	return pendingCandidates, data, err
}

func (p *PeerConnection) SetDescFromPeer(ansDesc []byte, candidates []string) error {
	answer := signaling.DescInfo{}
	if err := json.Unmarshal(ansDesc, &answer); err != nil {
		return err
	}
	if err := p.pc.SetRemoteDescription(answer.Sdp); err != nil {
		return err
	}
	for _, c := range candidates {
		if err := p.signal.SendICECandidate(answer.Name, c); err != nil {
			log.Println(err.Error())
			return err
		}
	}
	return nil
}

func (p *PeerConnection) SendDescToPeer(remote string, desc []byte) error {
	return p.signal.SendICESdp(remote, desc)
}

func (p *PeerConnection) OnRemoteDescription(onRecv signaling.OnDescReceived) {
	p.signal.AddICESdpListener(onRecv)
}

func (p *PeerConnection) Close() {
	p.signal.Disconnect()
	log.Println("signaling disconnected")
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
	go func() {
		p.pc.Close()
		cancel()
	}()
	<-ctx.Done()
	log.Println("pc disconnected")
}
