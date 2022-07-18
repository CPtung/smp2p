package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/CPtung/smp2p/pkg/offer"
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

	tcpClient := ssh.NewClient("127.0.0.1", 5566)
	if err := tcpClient.Bind(); err != nil {
		log.Println("create ssh client failed")
		return
	}

	tcpClient.Listen(offer.New)

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
}
