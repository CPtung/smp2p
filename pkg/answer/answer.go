package answer

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/CPtung/smp2p/internal/util"
	"github.com/CPtung/smp2p/pkg/ice"
	"github.com/CPtung/smp2p/pkg/mqtt"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/pion/webrtc/v3"
)

var target = "justin"

type AnswerImpl struct {
	ice.Desc
	mqttClient        MQTT.Client
	candidatesMux     sync.Mutex
	pendingCandidates []*webrtc.ICECandidate
	peerConnection    *webrtc.PeerConnection
}

func New(desc ice.Desc) ice.Answer {
	ans := AnswerImpl{
		Desc:              desc,
		mqttClient:        mqtt.NewClient(),
		pendingCandidates: make([]*webrtc.ICECandidate, 0),
	}
	if err := ans.Connect(); err != nil {
		log.Println("noooooot connected")
		return nil
	}
	// signaling server
	ans.AddSdpListener()
	ans.AddCandidateListener()

	// ICE
	ans.NewPeerConnection()

	return &ans
}

func (ans *AnswerImpl) Close() {
	if ans != nil {
		if ans.peerConnection != nil {
			if cErr := ans.peerConnection.Close(); cErr != nil {
				fmt.Printf("cannot close peerConnection: %v\n", cErr)
			}
		}
		ans.mqttClient.Disconnect(0)
	}
}

func (ans *AnswerImpl) AddCandidateListener() {
	onCandidateReceived :=
		func(client MQTT.Client, message MQTT.Message) {
			fmt.Printf("Received candidate on topic: %s\n", message.Topic())
			if candidateErr := ans.peerConnection.AddICECandidate(
				webrtc.ICECandidateInit{Candidate: string(message.Payload())}); candidateErr != nil {
				panic(candidateErr)
			}
		}

	candidateTopic := fmt.Sprintf("/candidate/%s/listen", ans.Name)
	if token := ans.mqttClient.Subscribe(candidateTopic, 0, onCandidateReceived); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
	}
}

func (ans *AnswerImpl) AddSdpListener() {

	onSdpReceived := func(client MQTT.Client, message MQTT.Message) {
		fmt.Printf("Received sdp on topic: %s\n", message.Topic())
		//info := ice.SdpInfo{}
		//json.Unmarshal(message.Payload(), &info)

		/*sdp := webrtc.SessionDescription{
			Type: webrtc.SDPTypeOffer,
			SDP:  string(message.Payload()),
		}*/
		sdp := webrtc.SessionDescription{}
		json.Unmarshal(message.Payload(), &sdp)
		if sdpErr := ans.peerConnection.SetRemoteDescription(sdp); sdpErr != nil {
			panic(sdpErr)
		}

		// Create an offer to send to the other process
		answer, err := ans.peerConnection.CreateAnswer(nil)
		if err != nil {
			panic(err)
		}

		// Send our offer to the HTTP signaling server listening in the other process
		payload, _ := json.Marshal(&answer)
		if err := ans.signalSdp(payload); err != nil {
			panic(err)
		}

		// Sets the LocalDescription, and starts our UDP listeners
		// Note: this will start the gathering of ICE candidates
		if err = ans.peerConnection.SetLocalDescription(answer); err != nil {
			panic(err)
		}

		ans.candidatesMux.Lock()
		for _, c := range ans.pendingCandidates {
			onICECandidateErr := ans.signalCandidate(c)
			if onICECandidateErr != nil {
				panic(onICECandidateErr)
			}
		}
		ans.candidatesMux.Unlock()

		log.Println("get offer!!!!!!!!!!")
	}

	sdpTopic := fmt.Sprintf("/sdp/%s/listen", ans.Name)
	if token := ans.mqttClient.Subscribe(sdpTopic, 0, onSdpReceived); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
	}
}

func (ans *AnswerImpl) Connect() error {
	// connect to singaling server (MQTT)
	fmt.Print("aaaaaaaa")
	if token := ans.mqttClient.Connect(); token.Wait() && token.Error() != nil {
		log.Printf("connect error: %s", token.Error())
		return token.Error()
	}
	fmt.Println("connect ok.....")
	return nil
}

func (ans *AnswerImpl) signalCandidate(c *webrtc.ICECandidate) error {
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

func (ans *AnswerImpl) NewPeerConnection() {
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
	ans.peerConnection, err = webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// When an ICE candidate is available send to the other Pion instance
	// the other Pion instance will add this candidate by calling AddICECandidate
	ans.peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		log.Println("on ice candidate")
		if c == nil {
			return
		}

		candidatesMux.Lock()
		defer candidatesMux.Unlock()

		desc := ans.peerConnection.RemoteDescription()
		if desc == nil {
			log.Printf("pendingCandidate: %v", c)
			pendingCandidates = append(pendingCandidates, c)
		} else {
			log.Printf("session: %s", desc)
			onICECandidateErr := ans.signalCandidate(c)
			if onICECandidateErr != nil {
				panic(onICECandidateErr)
			}
		}
	})

	ans.peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		// Register channel opening handling
		d.OnOpen(func() {
			fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", d.Label(), d.ID())
			for range time.NewTicker(5 * time.Second).C {
				message := util.RandSeq(15)
				fmt.Printf("Sending '%s'\n", message)

				// Send the message as text
				sendTextErr := d.SendText(message)
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
		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Printf("Message from DataChannel '%s': '%s'\n", d.Label(), string(msg.Data))
		})
	})
	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	ans.peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", s.String())

		if s == webrtc.PeerConnectionStateFailed {
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			fmt.Println("Peer Connection has gone to failed exiting")
			os.Exit(0)
		}
	})

	log.Println("finish answer.....")
}

func (ans *AnswerImpl) signalSdp(sdp []byte) error {
	sdpTopic := fmt.Sprintf("/sdp/%s/listen", target)
	if token := ans.mqttClient.Publish(sdpTopic, 0, false, sdp); token != nil && token.Error() != nil {
		return token.Error()
	}
	return nil
}
