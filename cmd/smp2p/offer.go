package main

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/CPtung/smp2p/pkg/peer"
	"github.com/CPtung/smp2p/pkg/session"
	"github.com/spf13/cobra"
)

var (
	command string
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
}

func offerRun(cmd *cobra.Command, args []string) {

	cmds := strings.Split(command, " ")
	if len(cmds) < 2 {
		log.Printf("incompleted command: %s", command)
		return
	}

	// Set remote peer name "leanne" for signaling handshake
	remote := "leanne"

	// Set local peer with name "justin" for signaling handshake
	pc := peer.Init("justin")
	if pc == nil {
		return
	}

	// Create PeerConnection
	if err := pc.Create(); err != nil {
		log.Printf("create session error: %s", err.Error())
		return
	}

	// Create TCP client session instance
	session := session.NewCli("0.0.0.0", dport)

	// Bind TCP session with local peer data channel
	if err := pc.BindOfferDataChannel(session); err != nil {
		log.Printf("create data session error: %s", err.Error())
		return
	}

	// Create local peer description and its ICE candidates
	candidates, offDesc, err := pc.CreateOfferDesc(remote)
	if err != nil {
		log.Printf("create description error: %s", err.Error())
		return
	}

	// Set remote peer description listener
	pc.OnRemoteDescription(func(desc []byte) {
		log.Println("get remote description")
		if err := pc.SetDescFromPeer(desc, candidates); err != nil {
			log.Printf("set remote description error: %s", err.Error())
		}
	})

	// Send local peer description to remote, this step will start p2p associating
	if err := pc.SendDescToPeer(remote, offDesc); err != nil {
		log.Printf("send description to peer error: %s", err.Error())
		return
	}

	// execute user command
	exeCmd := exec.Command(cmds[0], cmds[1:]...)
	// Set stdin to our connection
	exeCmd.Stdin = os.Stdin
	exeCmd.Stdout = os.Stdout
	exeCmd.Stderr = os.Stderr
	if err := exeCmd.Run(); err != nil {
		log.Printf("command error: %s", err.Error())
	}

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	pc.Close()
}
