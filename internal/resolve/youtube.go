package resolve

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	ytHost  = regexp.MustCompile(`(?i)^(www\.|m\.|music\.)?(youtube\.com|youtu\.be)$`)
	ytWatch = regexp.MustCompile(`(?i)(?:v=|/embed/|/shorts/|/live/|/v/)([A-Za-z0-9_-]{6,})`)
	ytBe    = regexp.MustCompile(`(?i)^/([A-Za-z0-9_-]{6,})`)
)

// YouTube resolves watch URLs from public page data (no Google API key).
type YouTube struct {
	client *http.Client
	ua     string
	log    *log.Logger
}

// Match detects youtube.com / youtu.be video URLs.
func (y *YouTube) Match(u *url.URL) bool {
	if u == nil || !ytHost.MatchString(u.Hostname()) {
		return false
	}
	return youtubeID(u) != ""
}

func youtubeID(u *url.URL) string {
	host := strings.ToLower(u.Hostname())
	if strings.HasSuffix(host, "youtu.be") {
		m := ytBe.FindStringSubmatch(u.Path)
		if len(m) == 2 {
			return m[1]
		}
		return ""
	}
	if v := u.Query().Get("v"); v != "" {
		return v
	}
	m := ytWatch.FindStringSubmatch(u.Path)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

// Resolve scrapes the public watch page (ytInitialPlayerResponse).
// oEmbed is last resort (title + channel only).
func (y *YouTube) Resolve(ctx context.Context, u *url.URL) (string, bool, error) {
	id := youtubeID(u)
	if id == "" {
		return "", false, nil
	}
	reply, ok, err := y.resolvePublicPage(ctx, id)
	if err == nil && ok {
		return reply, true, nil
	}
	if y.log != nil && err != nil {
		y.log.Printf("youtube page scrape: %v — falling back to oEmbed", err)
	}
	return y.resolveOEmbed(ctx, id)
}

// ytMeta is the normalised fields we print on IRC.
type ytMeta struct {
	Title    string
	Channel  string
	Duration string
	Uploaded string
	Views    string
	Likes    string
}

func (m ytMeta) Format() string {
	parts := []string{"YouTube: " + m.Title}
	if m.Channel != "" {
		parts = append(parts, m.Channel)
	}
	if m.Duration != "" && m.Duration != "?" {
		parts = append(parts, m.Duration)
	}
	if m.Uploaded != "" {
		parts = append(parts, "uploaded "+m.Uploaded)
	}
	if m.Views != "" {
		parts = append(parts, m.Views+" views")
	}
	if m.Likes != "" {
		parts = append(parts, m.Likes+" likes")
	}
	return strings.Join(parts, " | ")
}

// playerResponse is the subset of ytInitialPlayerResponse we care about.
type playerResponse struct {
	VideoDetails struct {
		Title         string `json:"title"`
		Author        string `json:"author"`
		LengthSeconds string `json:"lengthSeconds"`
		ViewCount     string `json:"viewCount"`
	} `json:"videoDetails"`
	Microformat struct {
		PlayerMicroformatRenderer struct {
			PublishDate      string `json:"publishDate"`
			UploadDate       string `json:"uploadDate"`
			LikeCount        string `json:"likeCount"`
			OwnerChannelName string `json:"ownerChannelName"`
		} `json:"playerMicroformatRenderer"`
	} `json:"microformat"`
}

func (y *YouTube) resolvePublicPage(ctx context.Context, id string) (string, bool, error) {
	watch := "https://www.youtube.com/watch?v=" + url.QueryEscape(id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, watch, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("User-Agent", y.ua)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := y.client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	// Watch pages are large; allow up to maxBody*2 for the HTML shell.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody*2))
	if err != nil {
		return "", false, err
	}
	raw, ok := extractJSONObject(string(body), "ytInitialPlayerResponse")
	if !ok {
		return "", false, fmt.Errorf("ytInitialPlayerResponse not found")
	}
	var pr playerResponse
	if err := json.Unmarshal(raw, &pr); err != nil {
		return "", false, err
	}
	vd := pr.VideoDetails
	mf := pr.Microformat.PlayerMicroformatRenderer
	channel := vd.Author
	if channel == "" {
		channel = mf.OwnerChannelName
	}
	uploaded := mf.PublishDate
	if uploaded == "" {
		uploaded = mf.UploadDate
	}
	sec, _ := strconv.Atoi(vd.LengthSeconds)
	meta := ytMeta{
		Title:    collapseSpace(vd.Title),
		Channel:  collapseSpace(channel),
		Duration: formatSeconds(sec),
		Uploaded: formatUploadDate(uploaded),
		Views:    compactCount(vd.ViewCount),
		Likes:    compactCount(mf.LikeCount),
	}
	if meta.Title == "" {
		return "", false, nil
	}
	return meta.Format(), true, nil
}

// extractJSONObject finds `name = {...}` / `name={...}` in HTML and returns the object bytes.
func extractJSONObject(html, name string) ([]byte, bool) {
	idx := strings.Index(html, name)
	if idx < 0 {
		return nil, false
	}
	rest := html[idx+len(name):]
	eq := strings.IndexByte(rest, '=')
	if eq < 0 {
		return nil, false
	}
	rest = rest[eq+1:]
	rest = strings.TrimLeft(rest, " \t\r\n")
	if rest == "" || rest[0] != '{' {
		return nil, false
	}
	depth := 0
	inStr := false
	esc := false
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if inStr {
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return []byte(rest[:i+1]), true
			}
		}
	}
	return nil, false
}

type oembedResp struct {
	Title      string `json:"title"`
	AuthorName string `json:"author_name"`
}

func (y *YouTube) resolveOEmbed(ctx context.Context, id string) (string, bool, error) {
	watch := "https://www.youtube.com/watch?v=" + url.QueryEscape(id)
	apiURL := "https://www.youtube.com/oembed?format=json&url=" + url.QueryEscape(watch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("User-Agent", y.ua)
	resp, err := y.client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return "", false, err
	}
	var parsed oembedResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", false, err
	}
	if parsed.Title == "" {
		return "", false, nil
	}
	meta := ytMeta{
		Title:   collapseSpace(parsed.Title),
		Channel: collapseSpace(parsed.AuthorName),
	}
	return meta.Format(), true, nil
}

func formatSeconds(sec int) string {
	if sec <= 0 {
		return ""
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func formatUploadDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	formats := []string{time.RFC3339, "2006-01-02T15:04:05-07:00", "2006-01-02"}
	for _, f := range formats {
		if t, err := time.Parse(f, raw); err == nil {
			return t.UTC().Format("2006-01-02")
		}
	}
	if len(raw) >= 10 {
		return raw[:10]
	}
	return raw
}

// compactCount turns "415854353" into "415.9M" for IRC-friendly lines.
func compactCount(raw string) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, ",", ""))
	if raw == "" {
		return ""
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil || n < 0 {
		return raw
	}
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", n/1_000_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", n/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", n/1_000)
	default:
		return strconv.FormatInt(int64(n), 10)
	}
}
