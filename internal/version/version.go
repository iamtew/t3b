// Package version holds the build stamp Meat Bags see in -version, CTCP, and UA.
//
// just build injects Version via -ldflags -X. Plain go build / go run leave it
// as "dev"; init then fills a short VCS hash from runtime/debug when available.
package version

import (
	"fmt"
	"runtime/debug"
)

// Version is the stamped build id (short hash, or tag-shorthash, maybe -dirty).
// Overridden at link time: -X github.com/iamtew/t3b/internal/version.Version=…
var Version = "dev"

func init() {
	if Version != "dev" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	var rev string
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if rev == "" {
		return
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	Version = rev
	if modified {
		Version += "-dirty"
	}
}

// String returns the CLI/display form, e.g. "t3b dc778dd".
func String() string {
	return fmt.Sprintf("t3b %s", Version)
}

// CTCP is the CTCP VERSION reply body.
func CTCP() string {
	return fmt.Sprintf("t3b %s (github.com/iamtew/t3b)", Version)
}

// UserAgent is the default HTTP User-Agent for resolvers.
// Mozilla-compatible bot form so CDNs (e.g. CloudFront on state.gov) that
// reject bare product tokens still serve real HTML; still identifies t3b.
func UserAgent() string {
	return fmt.Sprintf("Mozilla/5.0 (compatible; t3b/%s; +https://github.com/iamtew/t3b)", Version)
}
