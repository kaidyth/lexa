package p2p

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/apex/log"
	"github.com/kaidyth/lexa/shared/messages"
	"github.com/knadh/koanf"
	"github.com/perlin-network/noise"
	"github.com/perlin-network/noise/kademlia"
)

func NewNode(ctx context.Context) *noise.Node {
	k := ctx.Value("koanf").(*koanf.Koanf)
	bind := k.String("server.p2p.bind")
	bindAddr, _, _ := net.ParseCIDR(bind)
	port := uint16(k.Int("server.p2p.port"))

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

	node.RegisterMessage(messages.AgentInfoMessage{}, messages.UnmarshalAgentInfo)
	// Handle the inbound connection
	node.Handle(func(ctx noise.HandlerContext) error {
		// Ignore messages from self
		if node.ID().ID == ctx.ID().ID {
			return nil
		}

		if ctx.IsRequest() {
			return nil
		}

		obj, err := ctx.DecodeMessage()
		if err != nil {
			return nil
		}

		msg, ok := obj.(messages.AgentInfoMessage)
		if !ok {
			return nil
		}

		fmt.Printf("%s(%s)> %v\n", ctx.ID().Address, ctx.ID().ID.String()[:0], msg)

		// @TODO: Store data in local cache for DataProvider to pick up for DNS or https resolution
		return nil
	})

	// Setup peer discovery
	events := kademlia.Events{
		OnPeerAdmitted: func(id noise.ID) {
			log.Info(fmt.Sprintf("Learned about a new peer %s(%s).\n", id.Address, id.ID.String()))
		},
		OnPeerEvicted: func(id noise.ID) {
			log.Info(fmt.Sprintf("Forgotten a peer %s(%s).\n", id.Address, id.ID.String()))
		},
	}

	km := kademlia.New(kademlia.WithProtocolEvents(events))
	node.Bind(km.Protocol())
	err := node.Listen()
	km.Discover()

	// Create a scan interval to check for new nodes that connect
	peerScanInterval := k.Int("server.p2p.peerScanInterval")
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
				km.Discover()
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
