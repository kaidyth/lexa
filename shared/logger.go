package shared

import (
	"os"
	"strings"

	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
	"github.com/knadh/koanf"
)

// NewLogger returns a logger instance
func NewLogger(k *koanf.Koanf, mode string) {
	log.SetLevel(log.InfoLevel)
	level, err := log.ParseLevel(strings.ToLower(k.String(mode + ".log.level")))
	if err == nil {
		os.Exit(2)
	}

	log.SetLevel(level)

	// @TODO: Get log path and determine where to write out
	log.SetHandler(text.New(os.Stdout))
}
