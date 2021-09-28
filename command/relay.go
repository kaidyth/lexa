package command

import (
	"github.com/spf13/cobra"
)

var relayCmd = &cobra.Command{
	Use: "relay",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

func init() {
	rootCmd.AddCommand(relayCmd)
}
