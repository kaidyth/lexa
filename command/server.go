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
	"github.com/kaidyth/lexa/resolver"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/hcl"
	"github.com/knadh/koanf/providers/file"
	"github.com/miekg/dns"
	"github.com/spf13/cobra"
)

const WAITGROUP_INSTANCES = 3

var serverCmd = &cobra.Command{
	Use: "server",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		k := ctx.Value("koanf").(*koanf.Koanf)
		provider := ctx.Value("provider").(*file.File)

		// Create a new waitgroup to allow for paralleization of DNS and API response
		var wg sync.WaitGroup

		var httpServer = api.NewRouter(ctx)
		var dnsServer = resolver.NewResolver(ctx)
		var dotServer = resolver.NewDoTResolver(ctx)
		wg.Add(WAITGROUP_INSTANCES)

		startServers(k, &wg, ctx, provider, httpServer, dnsServer, dotServer)

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

	if err := api.Shutdown(ctx, httpServer); err != nil {
		log.Trace(fmt.Sprintf("HTTP server completed shutdown %v", err))
		wg.Done()
	}

	if err := resolver.Shutdown(ctx, dnsServer); err != nil {
		log.Trace(fmt.Sprintf("DNS server completed shutdown %v", err))
		wg.Done()
	}

	if err := resolver.Shutdown(ctx, dotServer); err != nil {
		log.Trace(fmt.Sprintf("DoT server completed shutdown %v", err))
		wg.Done()
	}

	*httpServer = *api.NewRouter(ctx)
	*dnsServer = *resolver.NewResolver(ctx)
	*dotServer = *resolver.NewDoTResolver(ctx)
	startServers(k, wg, ctx, provider, httpServer, dnsServer, dotServer)
}

func startServers(k *koanf.Koanf, wg *sync.WaitGroup, ctx context.Context, provider *file.File, httpServer *http.Server, dnsServer *dns.Server, dotServer *dns.Server) {

	// If the configuration file changes, shutdown the existing server
	// instances, then restart them with the new configuration within
	// a separate goroutine
	provider.Watch(func(event interface{}, err error) {
		k.Load(provider, hcl.Parser(true))
		reloadServers(k, ctx, wg, httpServer, dnsServer, dotServer, provider)
	})

	go api.StartServer(k, httpServer)
	go resolver.StartServer(dnsServer)
	go resolver.StartServer(dotServer)
}

func signalHandler(signal os.Signal, wg *sync.WaitGroup) {
	log.WithFields(log.Fields{
		"signal": fmt.Sprintf("%s", signal),
	}).Trace("Handling signal")

	switch signal {
	case syscall.SIGTERM:
	case syscall.SIGINT:
		wg.Add(-WAITGROUP_INSTANCES)
	case syscall.SIGHUP:

	default:
		fmt.Println("- unknown signal")
	}
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
