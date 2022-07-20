package mqtt

import (
	"fmt"
	"log"

	"github.com/CPtung/smp2p/pkg/signaling"
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

var sdpTopic = ""

var candidateTopic = ""

type Client struct {
	mq                MQTT.Client
	sdpSubTopic       string
	candidateSubTopic string
}

func (c *Client) Connect() error {
	if token := c.mq.Connect(); token.Wait() && token.Error() != nil {
		log.Printf("connect error: %s", token.Error())
		return token.Error()
	}
	return nil
}

func (c *Client) Disconnect() {
	c.mq.Disconnect(0)
}

func (c *Client) SendICESdp(remote string, sdp []byte) error {
	sdpTopic := fmt.Sprintf("/sdp/%s/listen", remote)
	if token := c.mq.Publish(sdpTopic, 0, false, sdp); token != nil && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (c *Client) SendICECandidate(remote, candidate string) error {
	candidateTopic := fmt.Sprintf("/candidate/%s/listen", remote)
	if token := c.mq.Publish(candidateTopic, 0, false, []byte(candidate)); token != nil && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (c *Client) AddICESdpListener(onSdp signaling.OnDescReceived) {
	onSdpReceived := func(client MQTT.Client, message MQTT.Message) {
		onSdp(message.Payload())
	}

	if token := c.mq.Subscribe(c.sdpSubTopic, 0, onSdpReceived); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
	}
}

func (c *Client) AddICECandidateListener(onCandidate signaling.OnCandidateReceived) {
	onCandidateReceived :=
		func(client MQTT.Client, message MQTT.Message) {
			fmt.Printf("Received candidate on topic: %s\n", message.Topic())
			onCandidate(message.Payload())
		}

	if token := c.mq.Subscribe(c.candidateSubTopic, 0, onCandidateReceived); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
	}
}

func New(name string) *Client {
	opts := MQTT.NewClientOptions()
	opts.AddBroker("tcp://test.mosquitto.org:1883")
	opts.ClientID = name

	return &Client{
		mq:                MQTT.NewClient(opts),
		sdpSubTopic:       fmt.Sprintf("/sdp/%s/listen", name),
		candidateSubTopic: fmt.Sprintf("/candidate/%s/listen", name),
	}
}
