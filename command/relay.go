package command

import (
	"fmt"

	"github.com/spf13/cobra"
)

var relayCmd = &cobra.Command{
	Use: "relay",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Create an ipfs relay")
	},
}

func init() {
	rootCmd.AddCommand(relayCmd)
}
