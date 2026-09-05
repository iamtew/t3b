package resolve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/iamtew/t3b/internal/config"
)

func TestFirstURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://example.com/page", "https://example.com/page"},
		{"see https://example.com/page. and more", "https://example.com/page"},
		{"https://example.com/page is cool", "https://example.com/page"},
		{"prefix https://example.com/page suffix", "https://example.com/page"},
		{"look <https://example.com/page>", "https://example.com/page"},
		{"https://example.com/page.", "https://example.com/page"},
		// Clients wrap inline links with mIRC attributes; strip before match.
		{"see \x1fhttps://example.com/page\x1f please", "https://example.com/page"},
		{"\x02https://example.com/page\x02 trailing", "https://example.com/page"},
		{"mid \x0304https://example.com/page\x03 text", "https://example.com/page"},
		{"mid \x0312,01https://example.com/page\x0f text", "https://example.com/page"},
		{"zw\u200bhttps://example.com/page more", "https://example.com/page"},
		{"no link here", ""},
		{"www.example.com/page", ""}, // scheme required
	}
	for _, tc := range cases {
		got := FirstURL(tc.in)
		if got != tc.want {
			t.Errorf("FirstURL(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestStripIRCFormatting(t *testing.T) {
	in := "a\x02b\x1fc\x0304d\x03e"
	got := stripIRCFormatting(in)
	if got != "abcde" {
		t.Fatalf("got %q", got)
	}
}

func TestURLTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title> Hello &amp; World </title></head></html>`))
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	r := &URLTitle{client: srv.Client(), ua: "t3b-test"}
	reply, ok, err := r.Resolve(context.Background(), u)
	if err != nil || !ok || !strings.Contains(reply, "Hello & World") {
		t.Fatalf("reply=%q ok=%v err=%v", reply, ok, err)
	}
}

func TestTwitterMatch(t *testing.T) {
	tr := &Twitter{}
	u, _ := url.Parse("https://x.com/someone/status/1234567890")
	if !tr.Match(u) {
		t.Fatal("expected match")
	}
	u2, _ := url.Parse("https://example.com/status/1")
	if tr.Match(u2) {
		t.Fatal("should not match")
	}
}

func TestTwitterResolveFixture(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/status/") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"code":200,"tweet":{"text":"hi","created_at":"Sun Jul 17 09:35:58 +0000 2022","author":{"screen_name":"bob"},"replies":1,"retweets":2,"likes":3,"media":{"photos":[{"url":"https://cdn.example/p.jpg"}]}}}`))
	}))
	defer srv.Close()

	// Point host via custom transport by rewriting — use Resolve against real fx path shape on test server.
	tr := &Twitter{client: srv.Client(), ua: "t3b-test"}
	orig := fxTwitterBase
	defer func() { /* can't reassign const */ }()
	_ = orig

	// Call JSON path indirectly: hit srv with same shape as Resolve builds.
	u, _ := url.Parse(srv.URL + "/bob/status/123")
	req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Unit-test Match + format via a thin override: parse fixture with Resolve by
	// temporarily using a Transport that redirects api.fxtwitter.com → srv.
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = strings.TrimPrefix(srv.URL, "http://")
			return http.DefaultTransport.RoundTrip(req)
		}),
	}
	tr.client = client
	status, _ := url.Parse("https://twitter.com/bob/status/123")
	reply, ok, err := tr.Resolve(context.Background(), status)
	if err != nil || !ok || !strings.Contains(reply, "@bob") || !strings.Contains(reply, "likes:3") {
		t.Fatalf("reply=%q ok=%v err=%v", reply, ok, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestYouTubeID(t *testing.T) {
	u, _ := url.Parse("https://youtu.be/dQw4w9WgXcQ")
	if youtubeID(u) != "dQw4w9WgXcQ" {
		t.Fatalf("id=%q", youtubeID(u))
	}
	u2, _ := url.Parse("https://www.youtube.com/watch?v=jNQXAC9IVRw")
	if youtubeID(u2) != "jNQXAC9IVRw" {
		t.Fatalf("id=%q", youtubeID(u2))
	}
}

const zooAPIJSON = `{"items":[{"snippet":{"title":"Me at the zoo","channelTitle":"jawed","publishedAt":"2005-04-23T20:31:52-07:00"},"contentDetails":{"duration":"PT19S"},"statistics":{"viewCount":"415854353","likeCount":"19591766"}}]}`

func rewriteToTestServer(srv *httptest.Server) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = strings.TrimPrefix(srv.URL, "http://")
		return http.DefaultTransport.RoundTrip(req)
	})
}

func TestYouTubeAPI(t *testing.T) {
	var gotKey, gotID bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") == "test-key" {
			gotKey = true
		}
		if r.URL.Query().Get("id") == "jNQXAC9IVRw" {
			gotID = true
		}
		if !strings.Contains(r.URL.Path, "/youtube/v3/videos") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(zooAPIJSON))
	}))
	defer srv.Close()

	y := &YouTube{
		ua:  "t3b-test",
		key: "test-key",
		client: &http.Client{
			Transport: rewriteToTestServer(srv),
		},
	}
	u, _ := url.Parse("https://youtu.be/jNQXAC9IVRw")
	reply, ok, err := y.Resolve(context.Background(), u)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v reply=%q", ok, err, reply)
	}
	for _, want := range []string{"Me at the zoo", "jawed", "19s", "415.9M views", "19.6M likes"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("missing %q in %q", want, reply)
		}
	}
	if !strings.Contains(reply, "uploaded 2005-04-") {
		t.Fatalf("missing upload date in %q", reply)
	}
	if !gotKey || !gotID {
		t.Fatalf("key=%v id=%v", gotKey, gotID)
	}
}

func TestYouTubeAPIEmptyItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()
	y := &YouTube{
		key:    "test-key",
		client: &http.Client{Transport: rewriteToTestServer(srv)},
	}
	u, _ := url.Parse("https://youtu.be/jNQXAC9IVRw")
	reply, ok, err := y.Resolve(context.Background(), u)
	if err != nil || ok || reply != "" {
		t.Fatalf("ok=%v err=%v reply=%q", ok, err, reply)
	}
}

func TestYouTubeAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":403,"message":"The request cannot be completed because you have exceeded your quota."}}`))
	}))
	defer srv.Close()
	y := &YouTube{
		key:    "test-key",
		client: &http.Client{Transport: rewriteToTestServer(srv)},
	}
	u, _ := url.Parse("https://youtu.be/jNQXAC9IVRw")
	_, _, err := y.Resolve(context.Background(), u)
	if err == nil || !strings.Contains(err.Error(), "quota") {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(err.Error(), "test-key") {
		t.Fatalf("API key leaked in error: %v", err)
	}
}

func TestYouTubeWithoutKeyUsesURLTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/youtube/v3/") {
			t.Errorf("Data API should not be called without a key")
			http.Error(w, "nope", http.StatusTeapot)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Me at the zoo - YouTube</title></head></html>`))
	}))
	defer srv.Close()

	on := true
	e := New(nil, config.Resolve{
		YouTube:        &on,
		URLTitles:      &on,
		UserAgent:      "t3b-test",
		HTTPTimeoutSec: 5,
	})
	e.client.Transport = rewriteToTestServer(srv)
	reply := e.HandleMessage(context.Background(), "#chan", "see https://youtu.be/jNQXAC9IVRw")
	if !strings.Contains(reply, "Title:") || !strings.Contains(reply, "Me at the zoo") {
		t.Fatalf("reply=%q", reply)
	}
	if strings.Contains(reply, "views") || strings.Contains(reply, "likes") {
		t.Fatalf("plain title should not have API details: %q", reply)
	}
}

func TestFormatISODuration(t *testing.T) {
	if got := formatISODuration("PT1M58S"); got != "1m58s" {
		t.Fatalf("got %q", got)
	}
	if got := formatISODuration("PT2H42M54S"); got != "2h42m54s" {
		t.Fatalf("got %q", got)
	}
	if got := formatISODuration("P1DT2H"); got != "26h0m0s" {
		t.Fatalf("got %q", got)
	}
}

func TestCompactCount(t *testing.T) {
	if got := compactCount("19591766"); got != "19.6M" {
		t.Fatalf("got %q", got)
	}
	if got := compactCount("42"); got != "42" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatSeconds(t *testing.T) {
	if got := formatSeconds(3723); got != "1h2m3s" {
		t.Fatalf("got %q", got)
	}
}
