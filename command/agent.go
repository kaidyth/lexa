package command

import (
	"github.com/kaidyth/lexa/ipfs"

	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use: "agent",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		_ = ipfs.NewIpfsAgent(ctx)
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)
}
