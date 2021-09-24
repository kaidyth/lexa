package command

import (
	"fmt"

	"github.com/apex/log"
	"github.com/knadh/koanf"

	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use: "agent",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		k := ctx.Value("koanf").(*koanf.Koanf)

		log.Debug(fmt.Sprintf("%v", k.Strings("agent.peers")))
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)
}
