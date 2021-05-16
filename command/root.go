package command

import (
	"context"

	"github.com/kaidyth/lexa/common"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/hcl"
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
		PersistentPreRun: persistentPreRun,
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

func persistentPreRun(cmd *cobra.Command, args []string) {
	provider = file.Provider(configFile)
	// Setup the initial logger to stdout with INFO level
	common.NewLogger(k)

	// Read the configuration file
	common.SetupConfig(k, provider)

	// Reload the logger configuration
	common.NewLogger(k)

	provider.Watch(func(event interface{}, err error) {
		// If the configuration file is changed, re-read the configuration file
		k.Load(provider, hcl.Parser(true))
		// and reload the logger configuration
		common.NewLogger(k)
	})
}
