package command

import (
	"context"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/eko/gocache/cache"
	"github.com/eko/gocache/store"
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

	// Don't leave your servers running for more than 5 years or you'll have a complete cache-eviction and Lexa will rebuild
	cacheClient, _ = bigcache.NewBigCache(bigcache.DefaultConfig(10 * time.Second))
	cacheStore     = store.NewBigcache(cacheClient, nil)
	cacheManager   = cache.New(cacheStore)
)

// Execute runs our root command
func Execute() error {
	cacheManager.Set("AllNodes", []byte("[]"), nil)

	ctx := context.Background()
	ctx = context.WithValue(ctx, "koanf", k)
	ctx = context.WithValue(ctx, "provider", provider)
	ctx = context.WithValue(ctx, "cache", cacheManager)

	return rootCmd.ExecuteContext(ctx)
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "lexa.hcl", "configuration file path")
}
