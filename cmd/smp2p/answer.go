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

var answerCmd = &cobra.Command{
	Use:   "answer",
	Short: "Run p2p answer",
	Run:   answerRun,
}

func init() {
	rootCmd.AddCommand(answerCmd)
}

func answerRun(cmd *cobra.Command, args []string) {
	pc := peer.Init("leanne")
	if pc == nil {
		return
	}
	pc.OnRemoteDescription(func(desc []byte) {
		log.Println("get remote sdp.....")

		if err := pc.CreateSession(); err != nil {
			log.Printf("create session error: %s", err.Error())
			return
		}
		if err := pc.CreateAnswerDataService(ssh.NewServ()); err != nil {
			log.Printf("create data service error: %s", err.Error())
			return
		}
		remote, ansDesc, err := pc.CreateAnswerDesc(desc)
		if err != nil {
			log.Printf("create description error: %s", err.Error())
			return
		}
		if err := pc.SendDescToPeer(remote, ansDesc); err != nil {
			log.Printf("send description to peer error: %s", err.Error())
			return
		}
	})
	log.Println("start waiting....")
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("interrupt.....")
	pc.Close()
	log.Println("close.....")
}
