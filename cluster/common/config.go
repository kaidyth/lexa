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
		"cluster.suffix":                  "lexa",
		"cluster.hotreload":               false,
		"cluster.backend.servers":         []string{},
		"cluster.backend.dnsServers":      []string{},
		"cluster.backend.insecure":        false,
		"cluster.lxd.http":                nil,
		"cluster.tls.bind":                "0.0.0.0",
		"cluster.tls.port":                18443,
		"cluster.tls.so_reuse_port":       false,
		"cluster.tls.certificate":         nil,
		"cluster.tls.key":                 nil,
		"cluster.dns.port":                18053,
		"cluster.dns.bind":                "0.0.0.0",
		"cluster.dns.tls.port":            18853,
		"cluster.dns.tls.bind":            "0.0.0.0",
		"cluster.dns.tls.certificate":     nil,
		"cluster.dns.tls.key":             nil,
		"cluster.tls.mtls.ca_certificate": nil,
		"cluster.log.level":               "trace",
		"cluster.log.path":                "stdout",
	}, "."), nil)

	if err := k.Load(provider, hcl.Parser(true)); err != nil {
		log.Error(fmt.Sprintf("Unable to read HCL configuration file: %v", err))
	}
}
