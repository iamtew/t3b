package bot

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/iamtew/t3b/internal/config"
	"github.com/lrstanley/girc"
)

func TestApplySASLPlain(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	gcfg := &girc.Config{}
	applySASL(logger, gcfg, config.SASL{Enabled: true, Mechanism: "PLAIN", User: "u", Password: "p"}, true)
	plain, ok := gcfg.SASL.(*girc.SASLPlain)
	if !ok || plain.User != "u" || plain.Pass != "p" {
		t.Fatalf("SASL not set: %#v", gcfg.SASL)
	}
	if !strings.Contains(buf.String(), "SASL PLAIN enabled") {
		t.Fatalf("log = %q", buf.String())
	}
}

func TestApplySASLDisabled(t *testing.T) {
	gcfg := &girc.Config{}
	applySASL(log.Default(), gcfg, config.SASL{Enabled: false}, true)
	if gcfg.SASL != nil {
		t.Fatal("expected nil SASL")
	}
}

func TestApplySASLWarnsWithoutTLS(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	gcfg := &girc.Config{}
	applySASL(logger, gcfg, config.SASL{Enabled: true, Mechanism: "plain", User: "u", Password: "p"}, false)
	if !strings.Contains(buf.String(), "without TLS") {
		t.Fatalf("expected TLS warning, got %q", buf.String())
	}
}
