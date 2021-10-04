package p2p

import (
	"context"
	"fmt"

	"github.com/apex/log"
	"github.com/perlin-network/noise"
	"github.com/perlin-network/noise/kademlia"
)

func NewNode(ctx context.Context) *noise.Node {
	node, err := noise.NewNode()
	if err == nil {
		return node
	}

	log.Error(fmt.Sprintf("Unable to start Noise P2P Server: %v", err))
	return nil
}

func StartServer(node *noise.Node) error {
	km := kademlia.New()
	node.Bind(km.Protocol())
	err := node.Listen()

	fmt.Printf(node.Addr())

	node.Ping(context.TODO(), "127.0.0.1")
	fmt.Printf("Node discovered %d peer(s).\n", len(km.Discover()))

	return err
}

func Shutdown(ctx context.Context, node *noise.Node) error {
	log.Info("Noise server shutdown")
	return node.Close()
}
