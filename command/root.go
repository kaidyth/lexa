package command

import (
	"context"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/providers/file"
	"github.com/spf13/cobra"
)

var (
	configFile string
	rootCmd    = &cobra.Command{
		Use:              "lexa",
		Short:            "Service and instance discovery for LXD",
		Long:             `Lexa providers service and instance discovery for LXD containers over an JSON REST API and over DNS.`,
		TraverseChildren: true,
	}
	k        = koanf.New(".")
	provider = file.Provider("lexa.hcl")
)

// Execute runs our root command
func Execute() error {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "koanf", k)
	ctx = context.WithValue(ctx, "provider", provider)
	return rootCmd.ExecuteContext(ctx)
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "lexa.hcl", "configuration file path")
}
