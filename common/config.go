package common

import (
	"github.com/apex/log"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/hcl"
	"github.com/knadh/koanf/providers/confmap"
)

func SetupConfig(k *koanf.Koanf, provider koanf.Provider) {
	k.Load(confmap.Provider(map[string]interface{}{
		"suffix":                  "lexa",
		"lxd.socket":              "/var/snap/lxd/common/lxd/unix.socket",
		"tls.bind":				   "0.0.0.0",
		"tls.port":                18433,
		"tls.so_reuse_port":       false,
		"tls.certificate":         nil,
		"tls.key":                 nil,
		"dns.port":                18053,
		"dns.bind":				   "0.0.0.0",
		"dns.tls.port":            18853,
		"dns.tls.bind": 		   "0.0.0.0",
		"dns.tls.certificate":     nil,
		"dns.tls.key":             nil,
		"tls.mtls.ca_certificate": nil,
		"log.level":               "trace",
		"log.path":                "stdout",
		"ipfs.seed":			   nil,
		"ipfs.bind":			   "0.0.0.0",
		"ipfs.port":			   9000,
	}, "."), nil)

	if err := k.Load(provider, hcl.Parser(true)); err != nil {
		log.Error("Unable to read HCL configuration file")
	}
}
