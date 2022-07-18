package offer

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/CPtung/smp2p/internal/util"
	"github.com/CPtung/smp2p/pkg/mqtt"
	"github.com/CPtung/smp2p/pkg/signal"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/pion/webrtc/v3"
)

var target = "leanne"

type OfferImpl struct {
	signal.Desc
	mqttClient        MQTT.Client
	candidatesMux     sync.Mutex
	pendingCandidates []*webrtc.ICECandidate
	peerConnection    *webrtc.PeerConnection
}

func New(desc signal.Desc) signal.Offer {
	offer := OfferImpl{
		Desc:              desc,
		mqttClient:        mqtt.NewClient(),
		pendingCandidates: make([]*webrtc.ICECandidate, 0),
	}
	// signaling server handshake
	if err := offer.Connect(); err != nil {
		return nil
	}
	offer.AddSdpListener()
	offer.AddCandidateListener()

	// webrtc handshake
	offer.NewPeerConnection()

	return &offer
}

func (off *OfferImpl) Close() {
	if off != nil {
		if cErr := off.peerConnection.Close(); cErr != nil {
			fmt.Printf("cannot close peerConnection: %v\n", cErr)
		}
		off.mqttClient.Disconnect(0)
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
			os.Exit(0)
		}
	})

	// Register channel opening handling
	dataChannel.OnOpen(func() {
		fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", dataChannel.Label(), dataChannel.ID())
		for range time.NewTicker(5 * time.Second).C {
			message := util.RandSeq(15)
			fmt.Printf("Sending '%s'\n", message)

			// Send the message as text
			sendTextErr := dataChannel.SendText(message)
			if sendTextErr != nil {
				if sendTextErr == io.ErrClosedPipe {
					break
				} else {
					panic(sendTextErr)
				}
			}
		}
	})

	// Register text message handling
	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		fmt.Printf("Message from DataChannel '%s': '%s'\n", dataChannel.Label(), string(msg.Data))
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
