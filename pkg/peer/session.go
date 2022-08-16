package p2p

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/CPtung/smp2p/pkg/model"
	"github.com/pion/webrtc/v3"
)

type Session interface {
	GetRemoteId() string
	DataProxy(conn net.Conn) error
	CreateDesc(remote *SessionDesc) (*SessionDesc, error)
	SendDesc(remoteId string, desc *SessionDesc) error
}

type ClientSession struct {
	localId          string
	remoteId         string
	peer             *Peer
	pc               *webrtc.PeerConnection
	candidateMutx    sync.Mutex
	candidates       []string
	remoteCandidates []string
	conn             net.Conn
}

type ServerSession struct {
	localId          string
	remoteId         string
	peer             *Peer
	pc               *webrtc.PeerConnection
	candidateMutx    sync.Mutex
	candidates       []string
	remoteCandidates []string
	conn             net.Conn
}

func (c *ClientSession) GetRemoteId() string {
	return c.remoteId
}

func (c *ClientSession) DataProxy(nc net.Conn) error {

	if c.pc == nil {
		return fmt.Errorf("peer connection is not created...")
	}

	c.conn = nc

	dc, err := c.pc.CreateDataChannel("data", nil)
	if err != nil {
		fmt.Printf("create data channel error: %s\n", err.Error())
		return err
	}

	// Register the OnMessage to handle incoming messages
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if _, err := c.conn.Write(msg.Data); err != nil {
			fmt.Printf("write data error: %s", err.Error())
		}
	})

	dc.OnOpen(func() {
		writer := &Wrap{DataChannel: dc}
		io.Copy(writer, c.conn)
		c.pc.Close()
		c.conn.Close()
	})
	return nil
}

func (c *ClientSession) CreateDesc(local *SessionDesc) (*SessionDesc, error) {
	// When an ICE candidate is available send to the other Pion instance
	// the other Pion instance will add this candidate by calling AddICECandidate
	c.pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		c.candidateMutx.Lock()
		desc := c.pc.RemoteDescription()
		if desc == nil {
			if candidate != nil {
				c.candidates = append(c.candidates, candidate.ToJSON().Candidate)
			}
		} else {
			fmt.Println("on ice candidate with remote description.....")
		}
		c.candidateMutx.Unlock()
	})

	c.peer.ForceSubscribe(GetP2PCandidateTopic(c.localId), func(topic string, candidate []byte) {
		if desc := c.pc.RemoteDescription(); desc != nil {
			if candidateErr := c.pc.AddICECandidate(
				webrtc.ICECandidateInit{Candidate: string(candidate)}); candidateErr != nil {
				fmt.Printf("Peer AddICECandidate error: %s\n", candidateErr.Error())
			}
		} else {
			c.remoteCandidates = append(c.remoteCandidates, string(candidate))
		}
	})

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	c.pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		if s == webrtc.PeerConnectionStateDisconnected {
			fmt.Printf("Peer Connection %s State has disconnected\n", c.localId)
			c.pc.Close()
		} else if s == webrtc.PeerConnectionStateFailed {
			fmt.Printf("Peer Connection %s State has failed\n", c.localId)
		} else {
			fmt.Printf("Peer Connection %s State has changed: %s\n", c.localId, s.String())
		}
	})

	// Create an offer to send to the other process
	offer, err := c.pc.CreateOffer(nil)
	if err != nil {
		return nil, err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	// Note: this will start the gathering of ICE candidates
	if err = c.pc.SetLocalDescription(offer); err != nil {
		return nil, err
	}

	// Send our offer to the HTTP signaling server listening in the other process
	local.Name = c.localId
	local.Sdp = offer
	return local, err
}

func (c *ClientSession) SendDesc(remoteId string, desc *SessionDesc) error {
	content, err := json.Marshal(desc)
	if err != nil {
		return err
	}
	msg := model.Message{
		Topic:   GetP2PSdpTopic(remoteId),
		QOS:     0,
		Retain:  false,
		Content: content,
	}
	if err := c.peer.ForcePublish(msg, false); err != nil {
		fmt.Println(err.Error())
		return err
	}
	return nil
}

func (c *ClientSession) OnSdpReceived(topic string, msg []byte) {
	answer := SessionDesc{}
	if err := json.Unmarshal(msg, &answer); err != nil {
		fmt.Printf("unmarshal answer sdp error: %s", err.Error())
		return
	}
	if err := c.pc.SetRemoteDescription(answer.Sdp); err != nil {
		fmt.Printf("set remote description error: %s", err.Error())
	}

	c.candidateMutx.Lock()
	defer c.candidateMutx.Unlock()

	for _, can := range c.remoteCandidates {
		if candidateErr := c.pc.AddICECandidate(
			webrtc.ICECandidateInit{Candidate: can}); candidateErr != nil {
			fmt.Printf("Peer AddICECandidate error: %s\n", candidateErr.Error())
		}
	}

	for _, can := range c.candidates {
		msg := model.Message{
			Topic:   GetP2PCandidateTopic(answer.Name),
			QOS:     0,
			Retain:  false,
			Content: []byte(can),
		}

		if err := c.peer.ForcePublish(msg, false); err != nil {
			fmt.Println(err.Error())
		}
	}
}

func NewClientSession(peer *Peer, group, device, clientId string, sessionId int) Session {
	s := ClientSession{
		localId:          fmt.Sprintf("%s/%s/%s%d", group, device, clientId, sessionId),
		remoteId:         fmt.Sprintf("%s/%s", group, device),
		peer:             peer,
		pc:               nil,
		candidates:       make([]string, 0),
		remoteCandidates: make([]string, 0),
	}
	s.pc, _ = NewPeerConnection(peer)
	s.peer.ForceSubscribe(GetP2PSdpTopic(s.localId), s.OnSdpReceived)
	return &s
}

// =========================================================

func (s *ServerSession) DataProxy(nc net.Conn) error {

	if s.pc == nil {
		return fmt.Errorf("peer connection is not created...")
	}

	s.conn = nc

	s.pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		// Register channel opening handling
		dc.OnOpen(func() {
			writer := &Wrap{DataChannel: dc}
			io.Copy(writer, s.conn)
			s.pc.Close()
			s.conn.Close()
		})

		// Register the OnMessage to handle incoming messages
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			if _, err := s.conn.Write(msg.Data); err != nil {
				fmt.Printf("write data error: %s", err.Error())
			}
		})

	})

	return nil
}

func (s *ServerSession) CreateDesc(remote *SessionDesc) (*SessionDesc, error) {
	if sdpErr := s.pc.SetRemoteDescription(remote.Sdp); sdpErr != nil {
		return nil, sdpErr
	}
	s.remoteId = remote.Name
	// Set ICE Candidate handler. As soon as a Peer has gathered a candidate
	// send it to the other peer
	s.pc.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i == nil {
			return
		}
		msg := model.Message{
			Topic:   GetP2PCandidateTopic(remote.Name),
			QOS:     0,
			Retain:  false,
			Content: []byte(i.ToJSON().Candidate),
		}
		if err := s.peer.ForcePublish(msg, false); err != nil {
			fmt.Println(err.Error())
		}
	})

	/*s.peer.ForceSubscribe(GetP2PCandidateTopic(s.localId), func(topic string, candidate []byte) {
		fmt.Println("server get remote candidates........")
		if candidateErr := s.pc.AddICECandidate(
			webrtc.ICECandidateInit{Candidate: string(candidate)}); candidateErr != nil {
			fmt.Printf("Peer AddICECandidate error: %s", candidateErr.Error())
		}
	})*/

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	s.pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateDisconnected {
			fmt.Printf("Peer Connection %s State has disconnected\n", s.localId)
		} else if state == webrtc.PeerConnectionStateFailed {
			fmt.Printf("Peer Connection %s State has failed\n", s.localId)
		} else {
			fmt.Printf("Peer Connection %s State has changed: %s\n", s.localId, state.String())
		}
	})

	answer, err := s.pc.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	if err := s.pc.SetLocalDescription(answer); err != nil {
		return nil, err
	}

	desc := SessionDesc{
		Name: s.localId,
		Sdp:  answer,
	}

	return &desc, err
}

func (s *ServerSession) GetRemoteId() string {
	return s.remoteId
}

func (s *ServerSession) SendDesc(remoteId string, desc *SessionDesc) error {
	content, err := json.Marshal(desc)
	if err != nil {
		return err
	}
	msg := model.Message{
		Topic:   GetP2PSdpTopic(s.remoteId),
		QOS:     0,
		Retain:  false,
		Content: content,
	}
	if err := s.peer.ForcePublish(msg, false); err != nil {
		fmt.Println(err.Error())
		return err
	}
	return nil
}

func NewServerSession(peer *Peer, group, device string) *ServerSession {
	s := ServerSession{
		localId:          fmt.Sprintf("%s/%s", group, device),
		peer:             peer,
		pc:               nil,
		candidates:       make([]string, 0),
		remoteCandidates: make([]string, 0),
	}
	s.pc, _ = NewPeerConnection(peer)
	return &s
}
