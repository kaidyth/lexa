package common

import (
	"fmt"

	"github.com/apex/log"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/hcl"
	"github.com/knadh/koanf/providers/confmap"
)

func SetupConfig(k *koanf.Koanf, provider koanf.Provider) {
	k.Load(confmap.Provider(map[string]interface{}{
		"server.suffix":                  "lexa",
		"server.lxd.socket":              "/var/snap/lxd/common/lxd/unix.socket",
		"server.lxd.http":                nil,
		"server.tls.bind":                "0.0.0.0",
		"server.tls.port":                18443,
		"server.tls.so_reuse_port":       false,
		"server.tls.certificate":         nil,
		"server.tls.key":                 nil,
		"server.dns.port":                18053,
		"server.dns.bind":                "0.0.0.0",
		"server.dns.tls.port":            18853,
		"server.dns.tls.bind":            "0.0.0.0",
		"server.dns.tls.certificate":     nil,
		"server.dns.tls.key":             nil,
		"server.tls.mtls.ca_certificate": nil,
		"server.p2p.bind":                "0.0.0.0",
		"server.p2p.port":                45861,
		"server.p2p.peerScanInterval":    5,
		"server.log.level":               "trace",
		"server.log.path":                "stdout",
	}, "."), nil)

	if err := k.Load(provider, hcl.Parser(true)); err != nil {
		log.Error(fmt.Sprintf("Unable to read HCL configuration file: %v", err))
	}
}
