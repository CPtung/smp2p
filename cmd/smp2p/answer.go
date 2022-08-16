package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CPtung/smp2p/pkg/mqtt"
	p2p "github.com/CPtung/smp2p/pkg/peer"
	"github.com/spf13/cobra"
)

var (
	answerPeer *p2p.Peer
	tunnelport int
)

var answerCmd = &cobra.Command{
	Use:   "answer",
	Short: "Run p2p answer",
	Run:   answerRun,
}

func init() {
	rootCmd.AddCommand(answerCmd)
	answerCmd.Flags().IntVarP(&sport, "port", "p", 22, "smp2p answer -p 22")
	answerCmd.Flags().IntVarP(&tunnelport, "tunnelport", "t", 0, "smp2p answer -t 40404")
	// Set remote peer name for signaling handshake
	answerCmd.Flags().StringVarP(&groupID, "group", "g", "demo", "local device group ID")
	answerCmd.Flags().StringVarP(&deviceID, "device", "d", "gateway", "local device ID")
}

func newSession(forward string) (net.Conn, error) {
	// connect to this socket
	local := "127.0.0.1:22"
	if forward != "" {
		local = forward
	}
	conn, err := net.DialTimeout("tcp", local, time.Duration(3)*time.Second)
	if err != nil {
		logger.Errorf("unable to connect to %s, err:%v", local, err)
		return nil, err
	}
	logger.Debugf("connected to addr:%s success", local)
	return conn, nil
}

func onP2PRemoteSessionReceived(topic string, data []byte) {
	logger.Info("OnP2PRemoteSessionReceived........")
	offer := p2p.SessionDesc{}
	if err := json.Unmarshal(data, &offer); err != nil {
		logger.Errorf("unmarshal p2p session description error: %s", err.Error())
		return
	}
	// create p2p instance
	pc := p2p.NewServerSession(answerPeer, groupID, deviceID)
	if pc == nil {
		logger.Errorf("create p2p server instance error.....")
		return
	}

	session, err := newSession(offer.ForwardAddr)
	if err != nil {
		logger.Warnln("failed to create session, err:", err)
	}

	// Bind TCP session with local peer data channel
	if err := pc.DataProxy(session); err != nil {
		fmt.Printf("create data service error: %s", err.Error())
		return
	}

	// Create local peer description
	ansDesc, err := pc.CreateDesc(&offer)
	if err != nil {
		fmt.Printf("create description error: %s", err.Error())
		return
	}

	// Response remote peer with local description
	if err := pc.SendDesc(offer.Name, ansDesc); err != nil {
		fmt.Printf("send description to peer error: %s", err.Error())
	}
}

func answerRun(cmd *cobra.Command, args []string) {
	var err error
	signalImpl := mqtt.New(fmt.Sprintf("%s-%s", groupID, deviceID))
	defer signalImpl.Disconnect()

	// Set local peer with name "leanne" for signaling handshake
	answerPeer, err = p2p.New(signalImpl, tunnelport)
	if err != nil {
		return
	}
	defer answerPeer.Close()

	topic := p2p.GetP2PSdpTopic(fmt.Sprintf("%s/%s", groupID, deviceID))
	answerPeer.ForceSubscribe(topic, onP2PRemoteSessionReceived)

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
}
