package ipfs

import (
	"context"
	"fmt"

	"github.com/apex/log"

	libp2p "github.com/libp2p/go-libp2p"
	host "github.com/libp2p/go-libp2p-core/host"
)

func NewIpfsHost(ctx context.Context) *host.Host {
	host, err := libp2p.New(ctx)
	if err != nil {
		panic(err)
	}

	log.Info(fmt.Sprintf("IPFS Server Created and started"))
	return &host
}

func Shutdown(ctx context.Context, host *host.Host) error {
	log.Info(fmt.Sprintf("IPFS Server Shutdown"))
	return (*host).Close()
}
