package mqtt

import (
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

func NewClient() MQTT.Client {
	opts := MQTT.NewClientOptions()
	opts.AddBroker("tcp://test.mosquitto.org:1883")
	opts.SetCleanSession(true)

	client := MQTT.NewClient(opts)
	return client
}
