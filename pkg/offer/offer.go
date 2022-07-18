package offer

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/CPtung/smp2p/pkg/mqtt"
	"github.com/CPtung/smp2p/pkg/signaling"
	"github.com/CPtung/smp2p/pkg/ssh"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/pion/webrtc/v3"
)

var target = "leanne"

type OfferImpl struct {
	signaling.Desc
	tcpConn           ssh.TcpConn
	mqttClient        MQTT.Client
	candidatesMux     sync.Mutex
	pendingCandidates []*webrtc.ICECandidate
	peerConnection    *webrtc.PeerConnection
}

func New(conn ssh.TcpConn) signaling.Offer {
	offer := OfferImpl{
		Desc:              signaling.Desc{Name: "justin"},
		tcpConn:           conn,
		mqttClient:        mqtt.NewClient(),
		pendingCandidates: make([]*webrtc.ICECandidate, 0),
	}
	// signaling server handshake
	if err := offer.Connect(); err != nil {
		return nil
	}
	offer.AddSdpListener()
	offer.AddCandidateListener()

	// start listening ssh client connect request
	offer.NewPeerConnection()
	return &offer
}

func (off *OfferImpl) Close() {
	if off != nil {
		if cErr := off.peerConnection.Close(); cErr != nil {
			fmt.Printf("cannot close peerConnection: %v\n", cErr)
		}
		off.mqttClient.Disconnect(0)
		off.tcpConn.Close()
	}
}

func (off *OfferImpl) AddCandidateListener() {
	onCandidateReceived :=
		func(client MQTT.Client, message MQTT.Message) {
			fmt.Printf("Received candidate on topic: %s\n", message.Topic())
			if candidateErr := off.peerConnection.AddICECandidate(
				webrtc.ICECandidateInit{Candidate: string(message.Payload())}); candidateErr != nil {
				panic(candidateErr)
			}
		}

	candidateTopic := fmt.Sprintf("/candidate/%s/listen", off.Name)
	if token := off.mqttClient.Subscribe(candidateTopic, 0, onCandidateReceived); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
	}
}

func (off *OfferImpl) AddSdpListener() {

	onSdpReceived := func(client MQTT.Client, message MQTT.Message) {
		fmt.Printf("Received sdp on topic: %s\n", message.Topic())
		sdp := webrtc.SessionDescription{}
		json.Unmarshal(message.Payload(), &sdp)
		if sdpErr := off.peerConnection.SetRemoteDescription(sdp); sdpErr != nil {
			panic(sdpErr)
		}

		off.candidatesMux.Lock()
		defer off.candidatesMux.Unlock()

		for _, c := range off.pendingCandidates {
			if onICECandidateErr := off.signalCandidate(c); onICECandidateErr != nil {
				panic(onICECandidateErr)
			}
		}
		log.Println("get answer!!!!!!!!!!")
	}

	sdpTopic := fmt.Sprintf("/sdp/%s/listen", off.Name)
	if token := off.mqttClient.Subscribe(sdpTopic, 0, onSdpReceived); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
	}
}

func (off *OfferImpl) Connect() error {
	// connect to singaling server (MQTT)
	if token := off.mqttClient.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		return token.Error()
	}
	return nil
}

func (off *OfferImpl) signalCandidate(c *webrtc.ICECandidate) error {
	payload := []byte(c.ToJSON().Candidate)
	candidateTopic := fmt.Sprintf("/candidate/%s/listen", target)
	if token := off.mqttClient.Publish(candidateTopic, 0, false, payload); token != nil && token.Error() != nil {
		return token.Error()
	}
	return nil
}

//*****************************************
// ICE functions
//*****************************************

func (off *OfferImpl) NewPeerConnection() {
	var err error
	var candidatesMux sync.Mutex
	pendingCandidates := make([]*webrtc.ICECandidate, 0)

	// Everything below is the Pion WebRTC API! Thanks for using it ❤️.

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	off.peerConnection, err = webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// When an ICE candidate is available send to the other Pion instance
	// the other Pion instance will add this candidate by calling AddICECandidate
	off.peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		log.Println("on ice candidate")
		if c == nil {
			return
		}

		candidatesMux.Lock()
		defer candidatesMux.Unlock()

		desc := off.peerConnection.RemoteDescription()
		if desc == nil {
			log.Printf("pendingCandidate: %v", c)
			pendingCandidates = append(pendingCandidates, c)
		} else {
			log.Printf("session: %s", desc)
			onICECandidateErr := off.signalCandidate(c)
			if onICECandidateErr != nil {
				panic(onICECandidateErr)
			}
		}
	})

	// Create a datachannel with label 'data'
	dataChannel, err := off.peerConnection.CreateDataChannel("data", nil)
	if err != nil {
		panic(err)
	}

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	off.peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", s.String())

		if s == webrtc.PeerConnectionStateFailed {
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			fmt.Println("Peer Connection has gone to failed exiting")
			off.Close()
		} else if s == webrtc.PeerConnectionStateDisconnected {
			fmt.Println("Peer Disconnected")
			off.Close()
		}
	})

	// Register channel opening handling
	dataChannel.OnOpen(func() {
		io.Copy(&signaling.Wrap{DataChannel: dataChannel}, off.tcpConn.Tx())
		log.Println("disconnected....")
		off.Close()
	})

	// Register text message handling
	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		if _, err := off.tcpConn.Rx(msg.Data); err != nil {
			log.Printf("on message error: %s", err.Error())
			off.Close()
		}
	})

	// Create an offer to send to the other process
	offer, err := off.peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	// Note: this will start the gathering of ICE candidates
	if err = off.peerConnection.SetLocalDescription(offer); err != nil {
		panic(err)
	}

	// Send our offer to the HTTP signaling server listening in the other process
	payload, _ := json.Marshal(offer)
	if err := off.signalSdp(payload); err != nil {
		panic(err)
	}
	log.Println("finish offer.....")
}

func (off *OfferImpl) signalSdp(sdp []byte) error {
	sdpTopic := fmt.Sprintf("/sdp/%s/listen", target)
	if token := off.mqttClient.Publish(sdpTopic, 0, false, sdp); token != nil && token.Error() != nil {
		return token.Error()
	}
	return nil
}
