package ipfs

import (
	"context"
	"fmt"
	"time"

	"github.com/apex/log"
	"github.com/knadh/koanf"

	libp2p "github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	host "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	noise "github.com/libp2p/go-libp2p-noise"
	libp2pquic "github.com/libp2p/go-libp2p-quic-transport"
	libp2ptls "github.com/libp2p/go-libp2p-tls"
)

func NewIpfsHost(ctx context.Context) *host.Host {
	k := ctx.Value("koanf").(*koanf.Koanf)
	port := k.String("ipfs.port")
	bind := k.String("ipfs.bind")

	priv, _, _ := crypto.GenerateKeyPair(
		crypto.Ed25519, // Select your key type. Ed25519 are nice short
		-1,             // Select key length when possible (i.e. RSA).
	)

	host, err := libp2p.New(ctx,
		libp2p.Identity(priv),
		// Multiple listen addresses
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/%s/tcp/%s", bind, port),
			fmt.Sprintf("/ip4/%s/udp/%s/quic", bind, port),
			fmt.Sprintf("/ip6/%s/tcp/%s", bind, port),
			fmt.Sprintf("/ip6/%s/udp/%s/quic", bind, port),
		),
		libp2p.Security(libp2ptls.ID, libp2ptls.New),
		libp2p.Security(noise.ID, noise.New),
		libp2p.Transport(libp2pquic.NewTransport),
		libp2p.DefaultTransports,
		libp2p.ConnectionManager(connmgr.NewConnManager(
			100,         // Lowwater
			400,         // HighWater,
			time.Minute, // GracePeriod
		)),
		libp2p.NATPortMap(),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			idht, err := dht.New(ctx, h)
			return idht, err
		}),
		libp2p.EnableAutoRelay(),
		libp2p.EnableNATService(),
	)

	log.Info(fmt.Sprintf("IPFS Server Created and started: %v", err))
	log.Trace(fmt.Sprintf("Host %s", host.ID()))
	return &host
}

func Shutdown(ctx context.Context, host *host.Host) error {
	log.Info(fmt.Sprintf("IPFS Server Shutdown"))
	return (*host).Close()
}
