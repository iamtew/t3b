package version

import (
	"strings"
	"testing"
)

func TestUserAgentMozillaCompatible(t *testing.T) {
	ua := UserAgent()
	if !strings.HasPrefix(ua, "Mozilla/5.0 (compatible; t3b/") {
		t.Fatalf("UserAgent=%q want Mozilla-compatible t3b token", ua)
	}
	if !strings.Contains(ua, "+https://github.com/iamtew/t3b") {
		t.Fatalf("UserAgent=%q missing repo URL", ua)
	}
}
