package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var verbose bool
var sport, dport int

var rootCmd = &cobra.Command{
	Use:   "smp2p",
	Short: "\nsmtunnl p2p utility\n",
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
