package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/CPtung/smp2p/pkg/ice"
	"github.com/CPtung/smp2p/pkg/offer"

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
	client := offer.New(
		ice.Desc{
			Name: "justin",
		},
	)
	if client == nil {
		log.Println("create offer failed")
		return
	}

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	client.Close()
}
