package p2p

import (
	"fmt"
	"log"
	"net"

	"github.com/pion/webrtc/v3"
)

type Wrap struct {
	*webrtc.DataChannel
}

func (rtc *Wrap) Write(data []byte) (int, error) {
	err := rtc.DataChannel.Send(data)
	return len(data), err
}

type Peer struct {
	Signal
	api    *webrtc.API
	events chan string
}

func newWebRtcAPI(port int) (*webrtc.API, error) {
	// Listen on UDP Port 8443, will be used for all WebRTC traffic
	udpListener, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.IP{0, 0, 0, 0},
		Port: port,
	})
	if err != nil {
		return nil, err
	}

	fmt.Printf("Listening for WebRTC traffic at %s\n", udpListener.LocalAddr())

	// Create a SettingEngine, this allows non-standard WebRTC behavior
	settingEngine := webrtc.SettingEngine{}

	// Configure our SettingEngine to use our UDPMux. By default a PeerConnection has
	// no global state. The API+SettingEngine allows the user to share state between them.
	// In this case we are sharing our listening port across many.
	settingEngine.SetICEUDPMux(webrtc.NewICEUDPMux(nil, udpListener))

	// Create a new API using our SettingEngine
	return webrtc.NewAPI(webrtc.WithSettingEngine(settingEngine)), nil
}

func New(signal Signal, port int) (*Peer, error) {

	var (
		err error
		api *webrtc.API
	)
	if port > 0 {
		api, err = newWebRtcAPI(port)
		if err != nil {
			log.Printf("bind udp port error: %s", err.Error())
			return nil, err
		}
	}
	return &Peer{
		api:    api,
		Signal: signal,
		events: make(chan string, 3),
	}, nil
}

func NewPeerConnection(peer *Peer) (pc *webrtc.PeerConnection, err error) {
	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}
	if peer.api == nil {
		return webrtc.NewPeerConnection(config)
	} else {
		return peer.api.NewPeerConnection(config)
	}
}

func (p *Peer) Close() {

}
