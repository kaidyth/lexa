package command

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/apex/log"
	"github.com/kaidyth/lexa/cluster/api"
	"github.com/kaidyth/lexa/cluster/common"
	"github.com/kaidyth/lexa/cluster/resolver"
	"github.com/kaidyth/lexa/shared"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/hcl"
	"github.com/knadh/koanf/providers/file"
	"github.com/miekg/dns"
	"github.com/spf13/cobra"
)

const CLUSTER_WAITGROUP_INSTANCES = 3

var clusterCmd = &cobra.Command{
	Use:              "cluster",
	PersistentPreRun: clusterPersistentPreRun,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		k := ctx.Value("koanf").(*koanf.Koanf)
		provider := ctx.Value("provider").(*file.File)

		// Create a new waitgroup to allow for paralleization of DNS and API response
		var wg sync.WaitGroup

		var dnsServer = resolver.NewResolver(ctx)
		var dotServer = resolver.NewDoTResolver(ctx)
		var httpServer = api.NewRouter(ctx)
		wg.Add(CLUSTER_WAITGROUP_INSTANCES)
		wg_count = CLUSTER_WAITGROUP_INSTANCES

		hotReload := k.Bool("cluster.hotreload")
		startClusterServers(k, &wg, ctx, provider, httpServer, dnsServer, dotServer, !hotReload)

		// Create a signal handler for TERM, INT, and USR1
		var captureSignal = make(chan os.Signal, 1)
		signal.Notify(captureSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGHUP)
		clusterSignalHandler(<-captureSignal, &wg)

		// Wait for the goroutines to clearnly exist before ending the server
		wg.Wait()
	},
}

func reloadClusterServers(k *koanf.Koanf, ctx context.Context, wg *sync.WaitGroup, httpServer *http.Server, dnsServer *dns.Server, dotServer *dns.Server, provider *file.File) {
	log.Info("Reloading server with updated configuration")
	wg.Add(CLUSTER_WAITGROUP_INSTANCES)
	wg_count += CLUSTER_WAITGROUP_INSTANCES

	if err := api.Shutdown(ctx, httpServer); err != nil {
		log.Trace(fmt.Sprintf("HTTP server shutdown error: %v", err))
	}

	if err := resolver.Shutdown(ctx, dnsServer); err != nil {
		log.Trace(fmt.Sprintf("DNS server shutdown error: %v", err))
	}

	if err := resolver.Shutdown(ctx, dotServer); err != nil {
		log.Trace(fmt.Sprintf("DoT server shutdown error: %v", err))
	}

	wg_count -= CLUSTER_WAITGROUP_INSTANCES
	wg.Done()
	wg.Done()
	wg.Done()

	*httpServer = *api.NewRouter(ctx)
	*dnsServer = *resolver.NewResolver(ctx)
	*dotServer = *resolver.NewDoTResolver(ctx)

	startClusterServers(k, wg, ctx, provider, httpServer, dnsServer, dotServer, true)
}

func startClusterServers(k *koanf.Koanf, wg *sync.WaitGroup, ctx context.Context, provider *file.File, httpServer *http.Server, dnsServer *dns.Server, dotServer *dns.Server, isWatching bool) {

	// If the configuration file changes, shutdown the existing server
	// instances, then restart them with the new configuration within
	// a separate goroutine
	if !isWatching {
		log.Debug("Creating new Watching Provider")
		provider.Watch(func(event interface{}, err error) {
			k.Load(provider, hcl.Parser(true))
			shared.NewLogger(k, "server")
			log.Debug("Watch event fired")
			ctx = context.WithValue(ctx, "koanf", k)
			ctx = context.WithValue(ctx, "provider", provider)
			reloadClusterServers(k, ctx, wg, httpServer, dnsServer, dotServer, provider)
		})
	}

	go api.StartServer(k, httpServer)
	go resolver.StartServer(dnsServer)
	go resolver.StartServer(dotServer)
}

func clusterSignalHandler(signal os.Signal, wg *sync.WaitGroup) {
	log.WithFields(log.Fields{
		"signal": fmt.Sprintf("%s", signal),
	}).Trace("Handling signal")

	log.Trace(fmt.Sprintf("Active Waitgroup Instances: %d", wg_count))
	switch signal {
	case syscall.SIGTERM:
		wg.Add(-CLUSTER_WAITGROUP_INSTANCES)
	case syscall.SIGINT:
		wg.Add(-CLUSTER_WAITGROUP_INSTANCES)
	default:
		fmt.Println("- unknown signal")
	}
}

func init() {
	rootCmd.AddCommand(clusterCmd)
}

func clusterPersistentPreRun(cmd *cobra.Command, args []string) {
	provider = file.Provider(configFile)
	// Setup the initial logger to stdout with INFO level
	shared.NewLogger(k, "cluster")

	// Read the configuration file
	common.SetupConfig(k, provider)

	// Reload the logger configuration
	shared.NewLogger(k, "cluster")
}
