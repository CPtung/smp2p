package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/CPtung/smp2p/pkg/answer"
	"github.com/CPtung/smp2p/pkg/ice"

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
	client := answer.New(
		ice.Desc{
			Name: "leanne",
		},
	)
	if client == nil {
		log.Println("create answer failed")
		return
	}

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	client.Close()
}
