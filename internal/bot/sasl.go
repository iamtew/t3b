package bot

import (
	"log"
	"strings"

	"github.com/iamtew/t3b/internal/config"
	"github.com/lrstanley/girc"
)

// applySASL wires girc SASL from config. Only PLAIN is supported (validated in config).
func applySASL(logger *log.Logger, gcfg *girc.Config, sasl config.SASL, tls bool) {
	if !sasl.Enabled {
		return
	}
	mech := strings.ToUpper(strings.TrimSpace(sasl.Mechanism))
	if mech == "" {
		mech = "PLAIN"
	}
	if mech != "PLAIN" {
		// Validate() should have caught this; refuse silently skipping.
		logger.Printf("SASL mechanism %q not supported — refusing to set girc.SASL", mech)
		return
	}
	if !tls {
		logger.Printf("warning: SASL PLAIN enabled without TLS — credentials may be exposed")
	}
	gcfg.SASL = &girc.SASLPlain{User: sasl.User, Pass: sasl.Password}
	logger.Printf("SASL PLAIN enabled for user %q", sasl.User)
}
