package command

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/apex/log"
	"github.com/kaidyth/lexa/agent/p2p"
	"github.com/kaidyth/lexa/server/common"
	"github.com/kaidyth/lexa/shared"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/hcl"
	"github.com/knadh/koanf/providers/file"
	"github.com/perlin-network/noise"
	"github.com/spf13/cobra"
)

const AGENT_WAITGROUP_INSTANCES = 1

var agentCmd = &cobra.Command{
	Use:              "agent",
	PersistentPreRun: agentPersistentPreRun,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		k := ctx.Value("koanf").(*koanf.Koanf)
		provider := ctx.Value("provider").(*file.File)

		// Create a new waitgroup to allow for paralleization of DNS and API response
		var wg sync.WaitGroup

		var noiseServer = p2p.NewNode(ctx)
		wg.Add(AGENT_WAITGROUP_INSTANCES)
		wg_count = AGENT_WAITGROUP_INSTANCES

		startAgentServers(k, &wg, ctx, provider, noiseServer, false)

		// Create a signal handler for TERM, INT, and USR1
		var captureSignal = make(chan os.Signal, 1)
		signal.Notify(captureSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGHUP)
		agentSignalHandler(<-captureSignal, &wg)

		// Wait for the goroutines to clearnly exist before ending the server
		wg.Wait()

		// Cleanup?
	},
}

func reloadAgentServers(k *koanf.Koanf, ctx context.Context, wg *sync.WaitGroup, noiseServer *noise.Node, provider *file.File) {
	if err := p2p.Shutdown(ctx, noiseServer); err != nil {
		log.Trace(fmt.Sprintf("Noise P2P server shutdown error: %v", err))
	}

	wg_count -= AGENT_WAITGROUP_INSTANCES
	wg.Done()

	*noiseServer = *p2p.NewNode(ctx)

	startAgentServers(k, wg, ctx, provider, noiseServer, true)
}

func startAgentServers(k *koanf.Koanf, wg *sync.WaitGroup, ctx context.Context, provider *file.File, noiseServer *noise.Node, isWatching bool) {
	// If the configuration file changes, shutdown the existing server
	// instances, then restart them with the new configuration within
	// a separate goroutine
	if !isWatching {
		log.Debug("Creating new Watching Provider")
		provider.Watch(func(event interface{}, err error) {
			k.Load(provider, hcl.Parser(true))
			shared.NewLogger(k)
			log.Debug("Watch event fired")
			ctx = context.WithValue(ctx, "koanf", k)
			ctx = context.WithValue(ctx, "provider", provider)
			reloadAgentServers(k, ctx, wg, noiseServer, provider)
		})
	}

	go p2p.StartServer(ctx, noiseServer)
}

func init() {
	rootCmd.AddCommand(agentCmd)
}

func agentPersistentPreRun(cmd *cobra.Command, args []string) {
	provider = file.Provider(configFile)
	// Setup the initial logger to stdout with INFO level
	shared.NewLogger(k)

	// Read the configuration file
	common.SetupConfig(k, provider)

	// Reload the logger configuration
	shared.NewLogger(k)
}

func agentSignalHandler(signal os.Signal, wg *sync.WaitGroup) {
	log.WithFields(log.Fields{
		"signal": fmt.Sprintf("%s", signal),
	}).Trace("Handling signal")

	log.Trace(fmt.Sprintf("Active Waitgroup Instances: %d", wg_count))
	switch signal {
	case syscall.SIGTERM:
		wg.Add(-AGENT_WAITGROUP_INSTANCES)
	case syscall.SIGINT:
		wg.Add(-AGENT_WAITGROUP_INSTANCES)
	default:
		fmt.Println("- unknown signal")
	}
}
