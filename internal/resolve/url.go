package resolve

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

const maxBody = 1 << 20 // 1 MiB

var titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// URLTitle fetches HTML and extracts <title>.
type URLTitle struct {
	client *http.Client
	ua     string
}

// Match accepts any http(s) URL (used after specialised resolvers).
func (u *URLTitle) Match(raw *url.URL) bool {
	return raw != nil && (raw.Scheme == "http" || raw.Scheme == "https")
}

// Resolve GETs the page and returns its title.
func (u *URLTitle) Resolve(ctx context.Context, raw *url.URL) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw.String(), nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("User-Agent", u.ua)

	resp, err := u.client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	// Non-2xx often means a WAF/CDN error page with a misleading <title>
	// (e.g. CloudFront "Technical Difficulties"). Do not announce those.
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxBody))
		return "", false, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if ct != "" && !strings.Contains(ct, "html") && !strings.Contains(ct, "text/") && !strings.Contains(ct, "xml") {
		return "", false, nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return "", false, err
	}
	title := extractTitle(string(body))
	if title == "" {
		return "", false, nil
	}
	return fmt.Sprintf("Title: %s", title), true, nil
}

func extractTitle(html string) string {
	m := titleRe.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	t := collapseSpace(htmlUnescape(m[1]))
	if !utf8.ValidString(t) {
		t = strings.ToValidUTF8(t, "")
	}
	return t
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func htmlUnescape(s string) string {
	replacer := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&apos;", "'",
		"&nbsp;", " ",
	)
	return replacer.Replace(s)
}
