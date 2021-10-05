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
		"agent.p2p.bootstrapPeers":   []string{},
		"agent.p2p.peerScanInterval": 5,
		"agent.p2p.bind":             "0.0.0.0",
		"agent.p2p.port":             45862,
	}, "."), nil)

	if err := k.Load(provider, hcl.Parser(true)); err != nil {
		log.Error(fmt.Sprintf("Unable to read HCL configuration file: %v", err))
	}
}
