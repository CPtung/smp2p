package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/CPtung/smp2p/pkg/peer"
	"github.com/CPtung/smp2p/pkg/session"
	"github.com/pion/webrtc/v3"
	"github.com/spf13/cobra"
)

var (
	ansUser    string
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
	answerCmd.Flags().StringVarP(&ansUser, "user", "u", "moxa", "")
}

func answerRun(cmd *cobra.Command, args []string) {
	var (
		err error
		api *webrtc.API
	)

	// Set local peer with name "leanne" for signaling handshake
	pc := peer.Init(ansUser)
	if pc == nil {
		return
	}

	if tunnelport > 0 {
		api, err = peer.NewWebRtcAPI(tunnelport)
		if err != nil {
			log.Printf("bind udp port error: %s", err.Error())
			return
		}
	}

	// Set remote peer description listener
	pc.OnRemoteDescription(func(desc []byte) {

		// When receiving a remote peer description, answer peer will start to create a p2p instance
		// for the following handshake
		if err := pc.Create(api); err != nil {
			log.Printf("create session error: %s", err.Error())
			return
		}

		// Create TCP server session instance
		session := session.NewServ("127.0.0.1", sport)

		// Bind TCP session with local peer data channel
		if err := pc.CreateAnswerDataService(session); err != nil {
			log.Printf("create data service error: %s", err.Error())
			return
		}

		// Create local peer description
		remote, ansDesc, err := pc.CreateAnswerDesc(desc)
		if err != nil {
			log.Printf("create description error: %s", err.Error())
			return
		}

		// Response remote peer with local description
		if err := pc.SendDescToPeer(remote, ansDesc); err != nil {
			log.Printf("send description to peer error: %s", err.Error())
			return
		}
	})

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	pc.Close()
}
