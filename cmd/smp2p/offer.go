package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/CPtung/smp2p/pkg/peer"
	"github.com/CPtung/smp2p/pkg/ssh"
	"github.com/spf13/cobra"
)

var offerCmd = &cobra.Command{
	Use:   "offer",
	Short: "Run p2p offer",
	Run:   offerRun,
}

func init() {
	rootCmd.AddCommand(offerCmd)
}

func offerRun(cmd *cobra.Command, args []string) {

	remote := "leanne"
	pc := peer.Init("justin")
	if pc == nil {
		return
	}
	if err := pc.CreateSession(); err != nil {
		log.Printf("create session error: %s", err.Error())
		return
	}
	if err := pc.CreateOfferDataService(ssh.NewCli()); err != nil {
		log.Printf("create data session error: %s", err.Error())
		return
	}
	pendingCandidates, offDesc, err := pc.CreateOfferDesc(remote)
	if err != nil {
		log.Printf("create description error: %s", err.Error())
		return
	}
	pc.OnRemoteDescription(func(desc []byte) {
		log.Println("get remote description")
		if err := pc.SetDescFromPeer(desc, pendingCandidates); err != nil {
			log.Printf("set remote description error: %s", err.Error())
		}
	})
	if err := pc.SendDescToPeer(remote, offDesc); err != nil {
		log.Printf("send description to peer error: %s", err.Error())
		return
	}

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	pc.Close()
}
