package command

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/apex/log"
	"github.com/kaidyth/lexa/api"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/providers/file"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use: "server",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		k := ctx.Value("koanf").(*koanf.Koanf)
		provider := ctx.Value("provider").(*file.File)

		// Create a new waitgroup to allow for paralleization of DNS and API response
		wg := new(sync.WaitGroup)
		wg.Add(2)

		// Load the initial API Server
		server := api.NewRouter(k, wg, ctx)

		// If the configuration file changes, shutdown the existing server
		// instances, then restart them with the new configuration within
		// a separate goroutine
		provider.Watch(func(event interface{}, err error) {
			log.Info("Reloading server with updated configuration")
			// Incriment the waitgroup for the new goroutine
			wg.Add(+1)

			// Shut down the old goroutine
			server.Shutdown(ctx)

			// Reload the HTTP server with the new configuration
			go func() {
				server = api.NewRouter(k, wg, ctx)
				api.StartServer(k, server)
			}()
		})

		// Spawn the API instance in a separate goroutine
		go func() {
			api.StartServer(k, server)
		}()

		// Create a signal handler for TERM, INT, and USR1
		var captureSignal = make(chan os.Signal, 1)
		signal.Notify(captureSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
		signalHandler(<-captureSignal, wg)

		// Wait for the goroutines to clearnly exist before ending the server
		wg.Wait()

		// Cleanup?
	},
}

func signalHandler(signal os.Signal, wg *sync.WaitGroup) {
	log.WithFields(log.Fields{
		"signal": fmt.Sprintf("%s", signal),
	}).Trace("Handling server Signal")

	switch signal {
	case syscall.SIGTERM:
	case syscall.SIGINT:
		wg.Add(-2)
	case syscall.SIGUSR1:
	default:
		fmt.Println("- unknown signal")
	}
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
