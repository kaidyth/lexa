package command

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	versionCmd = &cobra.Command{
		Use: "version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Lexa - instance & service discovery for LXD\n")
			fmt.Printf("Version: %s: %s\n", version, architecture)
		},
	}
	version      = "0.0.0"
	architecture = "Linux/amd64"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}
