package p2p

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/apex/log"
	"github.com/knadh/koanf"
	"github.com/perlin-network/noise"
	"github.com/perlin-network/noise/kademlia"
)

func NewNode(ctx context.Context) *noise.Node {
	k := ctx.Value("koanf").(*koanf.Koanf)
	bind := k.String("agent.p2p.bind")
	bindAddr, _, _ := net.ParseCIDR(bind)
	port := uint16(k.Int("agent.p2p.port"))

	node, err := noise.NewNode(
		noise.WithNodeBindPort(port),
		noise.WithNodeBindHost(bindAddr),
	)

	if err == nil {
		return node
	}

	log.Error(fmt.Sprintf("Unable to start Noise P2P Server: %v", err))
	return nil
}

func StartServer(ctx context.Context, node *noise.Node) error {
	k := ctx.Value("koanf").(*koanf.Koanf)

	// Setup peer discovery
	km := kademlia.New()
	node.Bind(km.Protocol())
	err := node.Listen()

	// Connect to our bootstrap nodes with a PING command
	peers := k.Strings("agent.p2p.bootstrapPeers")
	for _, peer := range peers {
		node.Ping(context.TODO(), peer)
	}
	km.Discover()

	// Create a scan interval to check for new nodes that connect
	peerScanInterval := k.Int("agent.p2p.peerScanInterval")
	if peerScanInterval <= 0 {
		peerScanInterval = 5
	}
	interval := time.Duration(peerScanInterval) * time.Second
	ticker := time.NewTicker(interval)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				log.Trace(fmt.Sprintf("Node discovered %d peer(s).\n", len(km.Discover())))
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	return err
}

func Shutdown(ctx context.Context, node *noise.Node) error {
	log.Info("Noise server shutdown")
	return node.Close()
}
