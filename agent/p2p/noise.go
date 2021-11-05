package p2p

import (
	"context"
	"fmt"
	"time"

	"github.com/apex/log"
	"github.com/kaidyth/lexa/shared/messages"
	"github.com/knadh/koanf"
	"github.com/perlin-network/noise"
	"github.com/perlin-network/noise/kademlia"
	"inet.af/netaddr"
)

func NewNode(ctx context.Context) *noise.Node {
	k := ctx.Value("koanf").(*koanf.Koanf)
	bind := k.String("agent.p2p.bind")
	ip, _ := netaddr.ParseIP(bind)
	bindAddr := ip.IPAddr().IP.To4()
	port := uint16(k.Int("agent.p2p.port"))
	listenAddress := ip.String() + ":" + fmt.Sprintf("%d", port)
	log.Info("Listening on: " + listenAddress)

	if bind == "" || ip.IsLoopback() || ip.IsMulticast() || ip.String() == "0.0.0.0" {
		log.Fatal(fmt.Sprintf("Unable to bind to (%s). Please use a non-local, non-multicast, and non 0.0.0.0 IP", ip.String()))
	}

	node, err := noise.NewNode(
		noise.WithNodeBindPort(port),
		noise.WithNodeBindHost(bindAddr),
		noise.WithNodeAddress(listenAddress),
		noise.WithNodeIdleTimeout(0),
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

	// Setup peer discovery
	events := kademlia.Events{
		OnPeerAdmitted: func(id noise.ID) {
			log.Info(fmt.Sprintf("Learned about a new peer %s%s(%s).\n", id.Host.String(), id.Address, id.ID.String()))
		},
		OnPeerEvicted: func(id noise.ID) {
			log.Info(fmt.Sprintf("Forgotten a peer %s%s(%s).\n", id.Host.String(), id.Address, id.ID.String()))
		},
	}

	km := kademlia.New(kademlia.WithProtocolEvents(events))
	node.Bind(km.Protocol())
	err := node.Listen()
	km.Discover()
	node.Bind(km.Protocol())
	node.Listen()

	// Connect to our bootstrap nodes with a PING command
	peers := k.Strings("agent.p2p.bootstrapPeers")
	for _, peer := range peers {
		_, err := node.Ping(context.TODO(), peer)
		if err != nil {
			log.Trace(fmt.Sprintf("Unable to connect to peer %s: %s", peer, err))
		}
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
				// Attempt to re-connect known peers each loop to enable persistent peer discovery
				for _, peer := range peers {
					node.Ping(context.TODO(), peer)
				}

				for _, id := range km.Discover() {
					var services []messages.Service
					k.Unmarshal("agent.service", &services)

					hostname := k.String("agent.p2p.hostname")
					message := messages.AgentInfoMessage{
						Name:     hostname,
						Services: services,
					}

					err := node.SendMessage(context.TODO(), id.Address, message)
					if err != nil {
						log.Debug(fmt.Sprintf("Unable to send message: %v", err))
					}
				}
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
