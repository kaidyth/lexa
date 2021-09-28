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
	"github.com/kaidyth/lexa/api"
	"github.com/kaidyth/lexa/common"
	"github.com/kaidyth/lexa/resolver"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/hcl"
	"github.com/knadh/koanf/providers/file"
	"github.com/miekg/dns"
	"github.com/spf13/cobra"
)

const WAITGROUP_INSTANCES = 3

var wg_count = 0
var serverCmd = &cobra.Command{
	Use: "server",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		k := ctx.Value("koanf").(*koanf.Koanf)
		provider := ctx.Value("provider").(*file.File)

		// Create a new waitgroup to allow for paralleization of DNS and API response
		var wg sync.WaitGroup

		var dnsServer = resolver.NewResolver(ctx)
		var dotServer = resolver.NewDoTResolver(ctx)
		var httpServer = api.NewRouter(ctx)
		wg.Add(WAITGROUP_INSTANCES)
		wg_count = WAITGROUP_INSTANCES

		startServers(k, &wg, ctx, provider, httpServer, dnsServer, dotServer, false)

		// Create a signal handler for TERM, INT, and USR1
		var captureSignal = make(chan os.Signal, 1)
		signal.Notify(captureSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGHUP)
		signalHandler(<-captureSignal, &wg)

		// Wait for the goroutines to clearnly exist before ending the server
		wg.Wait()

		// Cleanup?
	},
}

func reloadServers(k *koanf.Koanf, ctx context.Context, wg *sync.WaitGroup, httpServer *http.Server, dnsServer *dns.Server, dotServer *dns.Server, provider *file.File) {
	log.Info("Reloading server with updated configuration")
	wg.Add(WAITGROUP_INSTANCES)
	wg_count += WAITGROUP_INSTANCES

	if err := api.Shutdown(ctx, httpServer); err != nil {
		log.Trace(fmt.Sprintf("HTTP server shutdown error: %v", err))
	}

	if err := resolver.Shutdown(ctx, dnsServer); err != nil {
		log.Trace(fmt.Sprintf("DNS server shutdown error: %v", err))
	}

	if err := resolver.Shutdown(ctx, dotServer); err != nil {
		log.Trace(fmt.Sprintf("DoT server shutdown error: %v", err))
	}

	wg_count -= WAITGROUP_INSTANCES
	wg.Done()
	wg.Done()
	wg.Done()

	*httpServer = *api.NewRouter(ctx)
	*dnsServer = *resolver.NewResolver(ctx)
	*dotServer = *resolver.NewDoTResolver(ctx)
	startServers(k, wg, ctx, provider, httpServer, dnsServer, dotServer, true)
}

func startServers(k *koanf.Koanf, wg *sync.WaitGroup, ctx context.Context, provider *file.File, httpServer *http.Server, dnsServer *dns.Server, dotServer *dns.Server, isWatching bool) {

	// If the configuration file changes, shutdown the existing server
	// instances, then restart them with the new configuration within
	// a separate goroutine
	if !isWatching {
		log.Debug("Creating new Watching Provider")
		provider.Watch(func(event interface{}, err error) {
			k.Load(provider, hcl.Parser(true))
			common.NewLogger(k)
			log.Debug("Watch event fired")
			ctx = context.WithValue(ctx, "koanf", k)
			ctx = context.WithValue(ctx, "provider", provider)
			reloadServers(k, ctx, wg, httpServer, dnsServer, dotServer, provider)
		})
	}

	go api.StartServer(k, httpServer)
	go resolver.StartServer(dnsServer)
	go resolver.StartServer(dotServer)
}

func signalHandler(signal os.Signal, wg *sync.WaitGroup) {
	log.WithFields(log.Fields{
		"signal": fmt.Sprintf("%s", signal),
	}).Trace("Handling signal")

	log.Trace(fmt.Sprintf("Active Waitgroup Instances: %d", wg_count))
	switch signal {
	case syscall.SIGTERM:
		wg.Add(-WAITGROUP_INSTANCES)
	case syscall.SIGINT:
		wg.Add(-WAITGROUP_INSTANCES)
	default:
		fmt.Println("- unknown signal")
	}
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
