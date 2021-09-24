package ipfs

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/apex/log"
	"github.com/knadh/koanf"

	libp2p "github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	host "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	peerstore "github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	noise "github.com/libp2p/go-libp2p-noise"
	libp2pquic "github.com/libp2p/go-libp2p-quic-transport"
	libp2ptls "github.com/libp2p/go-libp2p-tls"
	ma "github.com/multiformats/go-multiaddr"
)

func NewIpfsHost(ctx context.Context) *host.Host {
	k := ctx.Value("koanf").(*koanf.Koanf)
	port := k.String("ipfs.port")
	bind := k.String("ipfs.bind")
	keyString := k.String("ipfs.privateKey")

	keyBytes, err := base64.StdEncoding.DecodeString(keyString)
	if err != nil {
		log.Fatal(fmt.Sprintf("Private key is formatted incorrectly"))
	}

	if len(keyBytes) != 64 {
		log.Fatal(fmt.Sprintf("Private key is incorrect length"))
	}

	priv, _ := crypto.UnmarshalEd25519PrivateKey(keyBytes)
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
	fullAddr := getHostAddress(host)
	log.Debug(fmt.Sprintf("I am %s\n", fullAddr))
	return &host
}

func NewIpfsAgent(ctx context.Context) *host.Host {
	k := ctx.Value("koanf").(*koanf.Koanf)

	priv, _, _ := crypto.GenerateKeyPair(
		crypto.Ed25519, // Select your key type. Ed25519 are nice short
		-1,             // Select key length when possible (i.e. RSA).
	)

	host, err := libp2p.New(ctx,
		libp2p.Identity(priv),
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
		libp2p.DisableRelay(),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			idht, err := dht.New(ctx, h)
			return idht, err
		}),
		libp2p.EnableNATService(),
	)

	log.Info(fmt.Sprintf("IPFS Server Created and started: %v", err))
	log.Trace(fmt.Sprintf("Host %s", host.ID()))

	for id, addresses := range k.StringsMap("agent.peer") {
		peer, err := peer.Decode(id)
		log.Debug(fmt.Sprintf("%v", err))
		for _, addr := range addresses {
			ipfsAddress, err := ma.NewMultiaddr(addr)
			log.Debug(fmt.Sprintf("%v", err))
			host.Peerstore().AddAddr(peer, ipfsAddress, peerstore.PermanentAddrTTL)
		}

	}

	return &host
}

func Shutdown(ctx context.Context, host *host.Host) error {
	log.Info(fmt.Sprintf("IPFS Server Shutdown"))
	return (*host).Close()
}

func getHostAddress(ha host.Host) string {
	// Build host multiaddress
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", ha.ID().Pretty()))

	// Now we can build a full multiaddress to reach this host
	// by encapsulating both addresses:
	addr := ha.Addrs()[0]
	return addr.Encapsulate(hostAddr).String()
}
