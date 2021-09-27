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

	var priv crypto.PrivKey
	keyBytes, err := base64.StdEncoding.DecodeString(keyString)
	if err != nil || len(keyBytes) != 68 {
		priv, _, _ = crypto.GenerateKeyPair(
			crypto.Ed25519,
			-1,
		)
	} else {
		priv, _ = crypto.UnmarshalPrivateKey(keyBytes)
	}

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
	fullAddr := GetHostAddresses(host)
	log.Trace(fmt.Sprintf("I am %v\n", fullAddr))

	return &host
}

func NewIpfsAgent(ctx context.Context) *host.Host {
	k := ctx.Value("koanf").(*koanf.Koanf)

	// Generates a random Keypair for this node
	priv, _, _ := crypto.GenerateKeyPair(
		crypto.Ed25519,
		-1,
	)

	var addrsd []ma.Multiaddr
	var peers []*peer.AddrInfo
	for _, addr := range k.Strings("agent.peer") {
		ipfsAddress, _ := ma.NewMultiaddr(addr)
		pi, _ := peer.AddrInfoFromP2pAddr(ipfsAddress)
		peers = append(peers, pi)
		addrsd = append(addrsd, ipfsAddress)
	}

	host, err := libp2p.New(ctx,
		libp2p.Identity(priv),
		libp2p.AddrsFactory(func(addrs []ma.Multiaddr) []ma.Multiaddr {
			return addrsd
		}),
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

	for _, pi := range peers {
		err := host.Connect(ctx, *pi)
		log.Debug(fmt.Sprintf("Connect: %v", err))
	}

	GetPeers(host)
	return &host
}

func Shutdown(ctx context.Context, host *host.Host) error {
	log.Info("IPFS Server Shutdown")
	return (*host).Close()
}

func GetPeers(ha host.Host) ([]*peer.AddrInfo, error) {
	var peers []*peer.AddrInfo
	for _, peer := range ha.Peerstore().PeersWithAddrs() {
		log.Debug(fmt.Sprintf("%v", peer.String()))
	}

	return peers, nil
}

func GetHostAddresses(ha host.Host) []string {
	// Build host multiaddress
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", ha.ID().Pretty()))

	// Now we can build a full multiaddress to reach this host
	// by encapsulating both addresses:
	var addrs []string
	for _, addr := range ha.Addrs() {
		addrs = append(addrs, addr.Encapsulate(hostAddr).String())
	}

	return addrs
}
