package ipfs

import (
	"context"
	"log"
	"time"

	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	routing "github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	libp2pquic "github.com/libp2p/go-libp2p-quic-transport"
	secio "github.com/libp2p/go-libp2p-secio"
	libp2ptls "github.com/libp2p/go-libp2p-tls"

	"crypto/tls"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/apex/log"
	"github.com/kaidyth/lexa/common"
	"github.com/knadh/koanf"

	"encoding/base64"
)

type struct IpfsKeypair {
	Private crypto.PrivateKey,
	Public crypto.PublicKey,
}

func NewIpfs(ctx context.Context) (IpfsKeypair) {
	k := ctx.Value("koanf").(*koanf.Koanf)
	seed := k.String("ipfs.seed")

	// If a seed isn't provided, generate one
	var pk
	var pub

	if (seed == nil) {
		pk, pub, err = crypto.GenerateKeyPair(
			crypto.Ed25519,
			-1,
		)

		// @todo: cleanup error handling
		if err != nil {
			log.Error("ipfs key generation failed.")
			panic(err)
		}
	} else {
		seedBytes, err := base64.StdEncoding.DecodeString(seed)
		if err != nil {
			log.Error("Unable to decode ipfs seed %s ", err.Error())
			panic(err)
		}
		pk = crypto.PrivateKey.NewKeyFromSeed(seedBytes)
		pub = pk.Public()
	}

	kp = IpfsKeypair{Public: pub, Private: pk}
	return kp

func StartServer(ctx *context.Context, pk crypto.PrivateKey, pub crypto.Public) error {
	var idht *dht.IpfsDHT
	bind := k.String("ipfs.bind")
	port := k.String("ipfs.port")

	host, err := libp2p.New(ctx,
		// Use the keypair we generated
		libp2p.Identity(priv),
		// Multiple listen addresses
		libp2p.ListenAddrStrings(
			"/ip4/" + bind + "/tcp/" + port,
			"/ip4/" + bind + "/udp/" + port + "/quic",
		),
		libp2p.Security(libp2ptls.ID, libp2ptls.New),
		libp2p.Security(secio.ID, secio.New),
		libp2p.Transport(libp2pquic.NewTransport),
		libp2p.DefaultTransports,
		libp2p.ConnectionManager(connmgr.NewConnManager(
			100,         // Lowwater
			400,         // HighWater,
			time.Minute, // GracePeriod
		)),
		libp2p.NATPortMap(),
		// Let this host use the DHT to find other hosts
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			idht, err = dht.New(ctx, h)
			return idht, err
		}),
		// Let this host use relays and advertise itself on relays if
		// it finds it is behind NAT. Use libp2p.Relay(options...) to
		// enable active relays and more.
		libp2p.EnableAutoRelay(),
		libp2p.EnableNATService(),
	)

	return host
}

func Shutdown(ctx *context.Context host *host.Host) error {
	return host.Close()
}