package answer

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/CPtung/smp2p/pkg/mqtt"
	"github.com/CPtung/smp2p/pkg/signaling"
	"github.com/CPtung/smp2p/pkg/ssh"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/pion/webrtc/v3"
)

var target = "justin"

type AnswerImpl struct {
	signaling.Desc
	mqttClient MQTT.Client
	session    *Session
}

func New(desc signaling.Desc) signaling.Answer {
	ans := AnswerImpl{
		Desc:       desc,
		mqttClient: mqtt.NewClient(),
	}

	if err := ans.Connect(); err != nil {
		log.Printf("connect signaling broker error: %s", err.Error())
		return nil
	}
	// signaling server
	ans.addSdpListener()
	ans.addCandidateListener()

	return &ans
}

func (ans *AnswerImpl) Close() {
	if ans != nil {
		ans.session.Close()
		ans.mqttClient.Disconnect(0)
	}
}

func (ans *AnswerImpl) addCandidateListener() {
	onCandidateReceived :=
		func(client MQTT.Client, message MQTT.Message) {
			fmt.Printf("Received candidate on topic: %s\n", message.Topic())
			if err := ans.session.AddICECandidate(string(message.Payload())); err != nil {
				fmt.Printf("AddICECandidate error: %s\n", err.Error())
			}
		}

	candidateTopic := fmt.Sprintf("/candidate/%s/listen", ans.Name)
	if token := ans.mqttClient.Subscribe(candidateTopic, 0, onCandidateReceived); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
	}
}

func (ans *AnswerImpl) addSdpListener() {

	onSdpReceived := func(client MQTT.Client, message MQTT.Message) {
		fmt.Printf("Received sdp on topic: %s\n", message.Topic())
		sdp := webrtc.SessionDescription{}
		if err := json.Unmarshal(message.Payload(), &sdp); err != nil {
			panic(err)
		}

		// Start new peerConnection
		session := NewSession(ans)
		session.Associate(sdp)
	}

	sdpTopic := fmt.Sprintf("/sdp/%s/listen", ans.Name)
	if token := ans.mqttClient.Subscribe(sdpTopic, 0, onSdpReceived); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
	}
}

func (ans *AnswerImpl) Connect() error {
	// connect to singaling server (MQTT)
	if token := ans.mqttClient.Connect(); token.Wait() && token.Error() != nil {
		log.Printf("connect error: %s", token.Error())
		return token.Error()
	}
	fmt.Println("connect ok.....")
	return nil
}

func (ans *AnswerImpl) SignalSdp(sdp []byte) error {
	sdpTopic := fmt.Sprintf("/sdp/%s/listen", target)
	if token := ans.mqttClient.Publish(sdpTopic, 0, false, sdp); token != nil && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (ans *AnswerImpl) SignalCandidate(c *webrtc.ICECandidate) error {
	payload := []byte(c.ToJSON().Candidate)
	candidateTopic := fmt.Sprintf("/candidate/%s/listen", target)
	if token := ans.mqttClient.Publish(candidateTopic, 0, false, payload); token != nil && token.Error() != nil {
		return token.Error()
	}
	return nil
}

//*****************************************
// ICE functions
//*****************************************

type Session struct {
	signaling.Answer
	candidatesMux     *sync.Mutex
	pendingCandidates []*webrtc.ICECandidate
	peerConnection    *webrtc.PeerConnection
	sock              ssh.TcpServer
}

func (s *Session) PeerConnection() *webrtc.PeerConnection {
	return s.peerConnection
}

func (s *Session) AddICECandidate(candidate string) error {
	if candidateErr := s.peerConnection.AddICECandidate(
		webrtc.ICECandidateInit{Candidate: candidate}); candidateErr != nil {
		return candidateErr
	}
	return nil
}

func (s *Session) Close() {
	if s.peerConnection != nil {
		if cErr := s.peerConnection.Close(); cErr != nil {
			fmt.Printf("cannot close peerConnection: %v\n", cErr)
		}
	}
}

func NewSession(sigClient signaling.Answer) *Session {
	// Everything below is the Pion WebRTC API! Thanks for using it ❤️.
	var (
		err               error
		sock              ssh.TcpServer
		peerConnection    *webrtc.PeerConnection
		candidatesMux     = &sync.Mutex{}
		pendingCandidates = make([]*webrtc.ICECandidate, 0)
	)

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err = webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// When an ICE candidate is available send to the other Pion instance
	// the other Pion instance will add this candidate by calling AddICECandidate
	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		log.Println("on ice candidate")
		if c == nil {
			return
		}
		candidatesMux.Lock()
		desc := peerConnection.RemoteDescription()
		if desc == nil {
			pendingCandidates = append(pendingCandidates, c)
		} else {
			onICECandidateErr := sigClient.SignalCandidate(c)
			if onICECandidateErr != nil {
				log.Printf("on ice candidate error: %s", onICECandidateErr.Error())
			}
		}
		candidatesMux.Unlock()
		log.Println("candidates end.....")
	})

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", s.String())
		if s == webrtc.PeerConnectionStateFailed {
			fmt.Println("Peer Connection has gone to failed exiting")
		}
	})

	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		sock = ssh.NewServer("127.0.0.1", 2222)
		if err := sock.Bind(); err != nil {
			log.Printf("bind tcp server error: %s", err.Error())
			return
		}
		d.OnOpen(func() {
			log.Println("OnOpen.....")
			log.Println("create session")
			sock.Tx(&signaling.Wrap{DataChannel: d})
			log.Println("tcp tx close, disconnected")
		})

		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			log.Println("OnMessage.....")
			_, err := sock.Rx(msg.Data)
			if err != nil {
				log.Printf("tcp rx failed: %s", err)
				return
			}
		})

		d.OnClose(func() {
			log.Println("OnClose.....")
			d.Close()
			sock.Close()
			peerConnection.Close()
			log.Println("on close.........")
		})
	})

	return &Session{
		sock:              sock,
		peerConnection:    peerConnection,
		candidatesMux:     candidatesMux,
		pendingCandidates: pendingCandidates,
		Answer:            sigClient,
	}
}

func (s *Session) Associate(sdp webrtc.SessionDescription) {
	if sdpErr := s.peerConnection.SetRemoteDescription(sdp); sdpErr != nil {
		panic(sdpErr)
	}

	// Create an offer to send to the other process
	answer, err := s.peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Send our offer to the HTTP signaling server listening in the other process
	payload, _ := json.Marshal(&answer)
	if err := s.SignalSdp(payload); err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	// Note: this will start the gathering of ICE candidates
	if err = s.peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	s.candidatesMux.Lock()
	defer s.candidatesMux.Unlock()
	for _, c := range s.pendingCandidates {
		onICECandidateErr := s.SignalCandidate(c)
		if onICECandidateErr != nil {
			panic(onICECandidateErr)
		}
	}
}
