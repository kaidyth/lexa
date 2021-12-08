package p2p

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apex/log"
	"github.com/eko/gocache/cache"
	"github.com/eko/gocache/store"
	"github.com/kaidyth/lexa/shared"
	"github.com/kaidyth/lexa/shared/messages"
	"github.com/knadh/koanf"
	"github.com/perlin-network/noise"
	"github.com/perlin-network/noise/kademlia"
)

func NewNode(ctx context.Context) *noise.Node {
	ip, port, err := shared.GetNetworkBindings(ctx, "server.p2p")
	if err != nil {
		log.Fatal("Unable to aquire bind")
	}

	bindAddr := ip.IPAddr().IP.To4()
	listenAddress := ip.String() + ":" + fmt.Sprintf("%d", port)
	log.Info("Listening on: " + listenAddress)

	if ip.IsLoopback() || ip.IsMulticast() || ip.String() == "0.0.0.0" {
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

	log.Fatal(fmt.Sprintf("Unable to start Noise P2P Server: %v", err))
	return nil
}

func StartServer(ctx context.Context, node *noise.Node) error {
	k := ctx.Value("koanf").(*koanf.Koanf)
	cacheManager := ctx.Value("cache").(*cache.Cache)

	// Create a scan interval to check for new nodes that connect
	peerScanInterval := k.Int("server.p2p.peerScanInterval")
	if peerScanInterval <= 0 {
		peerScanInterval = 5
	}

	node.RegisterMessage(messages.AgentInfoMessage{}, messages.UnmarshalAgentInfo)
	// Handle the inbound connection
	node.Handle(func(ncxt noise.HandlerContext) error {
		// Ignore messages from self
		if node.ID().ID == ncxt.ID().ID {
			return nil
		}

		if ncxt.IsRequest() {
			return nil
		}

		obj, err := ncxt.DecodeMessage()
		if err != nil {
			return nil
		}

		msg, ok := obj.(messages.AgentInfoMessage)
		if !ok {
			return nil
		}

		allNodes := getAllNodes(cacheManager)
		_, found := shared.Find(allNodes, msg.Name)
		if !found {
			allNodes = append(allNodes, msg.Name)
			encoded, _ := json.Marshal(allNodes)
			cacheManager.Set("AllNodes", encoded, nil)
		}

		data, err := json.Marshal(msg)
		if err == nil {
			// Store this in the cache for the peerScanInterval + .33 second for overhead
			options := &store.Options{
				Expiration: time.Duration(peerScanInterval) + (time.Second / 3),
			}
			// Store the node ID in the cache with a reference to the agent name
			cacheManager.Set(ncxt.ID().ID.String(), []byte(msg.Name), options)
			cacheManager.Set(msg.Name, data, options)
		}

		return nil
	})

	// Setup peer discovery
	events := kademlia.Events{
		OnPeerAdmitted: func(id noise.ID) {
			log.Info(fmt.Sprintf("Learned about a new peer %s(%s).\n", id.Address, id.ID.String()))
		},
		OnPeerEvicted: func(id noise.ID) {
			log.Info(fmt.Sprintf("Forgotten a peer %s(%s).\n", id.Address, id.ID.String()))

			// Get the cache key for the node
			var data []byte
			rawData, err := cacheManager.Get(id.ID.String())
			if err == nil {
				data = rawData.([]byte)
				name := bytes.NewBuffer(data).String()

				if err == nil {
					// Delete the node name
					cacheManager.Delete(name)

					// Delete the id
					cacheManager.Delete(id.ID.String())

					// Remove the node from AllNodes
					allNodes := getAllNodes(cacheManager)
					i, found := shared.Find(allNodes, name)
					if found {
						allNodes = removeIndex(allNodes, i)
						encoded, _ := json.Marshal(allNodes)
						cacheManager.Set("AllNodes", encoded, nil)
					}
				}
			}
		},
	}

	km := kademlia.New(kademlia.WithProtocolEvents(events))
	node.Bind(km.Protocol())
	err := node.Listen()
	km.Discover()

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

func getAllNodes(cacheManager *cache.Cache) []string {
	var allNodes []string
	// Put the element in the all nodes list if it isn't found.
	// This is non-atomic, and is eventually consistent with multi-nodes
	// @TODO: implement an atomic insert
	allNodesRaw, _ := cacheManager.Get("AllNodes")
	allNodesRawBytes, _ := allNodesRaw.([]byte)
	_ = json.Unmarshal(allNodesRawBytes, &allNodes)

	return allNodes
}

func removeIndex(s []string, index int) []string {
	return append(s[:index], s[index+1:]...)
}
