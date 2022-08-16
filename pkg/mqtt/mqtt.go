package mqtt

import (
	"errors"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/CPtung/smp2p/pkg/model"
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

var (
	sdpTopic       = ""
	candidateTopic = ""
	log            = logrus.WithField("origin", "mqtt")
)

type Connection struct {
	lock              sync.Mutex
	client            MQTT.Client
	sdpSubTopic       string
	candidateSubTopic string
}

func (c *Connection) Connect() error {
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		log.Printf("connect error: %s", token.Error())
		return token.Error()
	}
	return nil
}

func (c *Connection) Disconnect() {
	c.client.Disconnect(0)
}

func New(name string) *Connection {
	opts := MQTT.NewClientOptions()
	opts.AddBroker("tcp://test.mosquitto.org:1883")
	opts.ClientID = name

	connect := &Connection{
		client:            MQTT.NewClient(opts),
		sdpSubTopic:       fmt.Sprintf("/%s/sdp", name),
		candidateSubTopic: fmt.Sprintf("/%s/candidate", name),
	}
	if err := connect.Connect(); err != nil {
		log.Errorf("mqtt connection error: %s", err.Error())
		return nil
	}
	return connect
}

func (c *Connection) ForceSubscribe(topic string, handler func(topic string, data []byte)) {
	callback := func(client MQTT.Client, msg MQTT.Message) {
		handler(msg.Topic(), msg.Payload())
	}
	token := c.client.Subscribe(topic, byte(0), callback)
	if !token.Wait() {
		log.Errorln("subscribe token wait failure")
	}
	if err := token.Error(); err != nil {
		log.Errorln("subscribe fail, err:", err)
	}
}

func (c *Connection) ForcePublish(msg model.Message, compress bool) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	payload := msg.Content
	log.Debugln("@@ out >>", msg.Topic, len(msg.Content), len(payload))
	token := c.client.Publish(msg.Topic, byte(msg.QOS), msg.Retain, payload)
	token.Wait()
	if !token.Wait() {
		log.Errorln("token wait failure")
		return errors.New("token wait failure")
	}
	if err := token.Error(); err != nil {
		log.Errorln("publish get fail token, err:", err)
		return fmt.Errorf("publish get fail token, err: %s", err.Error())
	}
	return nil
}
