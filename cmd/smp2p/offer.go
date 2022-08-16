package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/CPtung/smp2p/pkg/mqtt"
	p2p "github.com/CPtung/smp2p/pkg/peer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	command     = ""
	groupID     = ""
	deviceID    = ""
	clientID    = ""
	remote      = ""
	forwardAddr = ""
	localAddr   = "127.0.0.1:6601"
	logger      = logrus.WithField("origin", "p2p")
)

var offerCmd = &cobra.Command{
	Use:   "offer",
	Short: "Run p2p offer",
	Run:   offerRun,
}

func init() {
	rootCmd.AddCommand(offerCmd)
	offerCmd.Flags().StringVarP(&command, "command", "c", "", "smp2p offer -c \"ssh moxa@127.0.0.1 -p ${LOCAL_PORT}\"")
	offerCmd.Flags().IntVarP(&dport, "port", "p", 5566, "smp2p offer -p 5566")
	// Set remote peer name for signaling handshake
	offerCmd.Flags().StringVarP(&groupID, "group", "g", "demo", "remote device group ID")
	offerCmd.Flags().StringVarP(&deviceID, "device", "d", "gateway", "remote device ID")
	// Set local peer name for signaling handshake
	offerCmd.Flags().StringVarP(&clientID, "user", "u", "justin", "current user ID")
	offerCmd.Flags().StringVarP(&forwardAddr, "forwardAddr", "f", "127.0.0.1:22", "remote forward address")
}

func proxyAccessment(ctx context.Context, peer *p2p.Peer) net.Listener {
	// bind local server
	listener, err := net.Listen("tcp", localAddr)
	if err != nil {
		logger.Errorln(err.Error())
		return nil
	}
	logger.Infoln("Proxy server start and listening on:", localAddr)

	// waiting client ------------------------------------------------------------
	go func() {
		var index int = 0
		for ctx.Err() == nil {
			sessionConn, err := listener.Accept()
			if ctx.Err() != nil {
				return
			}
			if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			pc := p2p.NewClientSession(peer, groupID, deviceID, clientID, index)
			if err != nil {
				logger.Warnln("failed to accept client connection, err:", err)
				return
			}
			if err := pc.DataProxy(sessionConn); err != nil {
				logger.Errorf("create data session error: %s\n", err.Error())
				return
			}
			sd := p2p.SessionDesc{
				ForwardAddr: forwardAddr,
			}
			// prepare p2p client sdp
			desc, err := pc.CreateDesc(&sd)
			if err != nil {
				logger.Errorf("create p2p client sdp error: %s", err.Error())
				return
			}
			if err := pc.SendDesc(pc.GetRemoteId(), desc); err != nil {
				logger.Errorf("create p2p client sdp error: %s", err.Error())
				return
			}
			index++
		}
	}()
	return listener
}

func offerRun(cmd *cobra.Command, args []string) {
	cmds := strings.Split(command, " ")
	if len(cmds) < 2 {
		logger.Printf("incompleted command: %s", command)
		return
	}

	signalImpl := mqtt.New(fmt.Sprintf("%s-%s-%s", groupID, deviceID, clientID))
	defer signalImpl.Disconnect()

	// Set local peer with name "leanne" for signaling handshake
	pc, err := p2p.New(signalImpl, 0)
	if err != nil {
		return
	}
	defer pc.Close()

	listener := proxyAccessment(cmd.Context(), pc)
	if listener == nil {
		return
	}
	defer listener.Close()

	// execute user command
	exeCmd := exec.Command(cmds[0], cmds[1:]...)
	// Set stdin to our connection
	exeCmd.Stdin = os.Stdin
	exeCmd.Stdout = os.Stdout
	exeCmd.Stderr = os.Stderr
	if err := exeCmd.Run(); err != nil {
		logger.Printf("command error: %s", err.Error())
	}
}
