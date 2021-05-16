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
		"tls.port":                18433,
		"tls.so_reuse_port":       false,
		"tls.certificate":         nil,
		"tls.key":                 nil,
		"tls.mtls.ca_certificate": nil,
		"log.level":               "trace",
		"log.path":                "stdout",
	}, "."), nil)

	if err := k.Load(provider, hcl.Parser(true)); err != nil {
		log.Error("Unable to read HCL configuration file")
	}
}
