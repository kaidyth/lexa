package command

import (
	"fmt"

	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use: "agent",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Agent: AAAA")
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)
}
