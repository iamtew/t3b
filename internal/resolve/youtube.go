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
	"strings"
	"time"
)

var (
	ytHost  = regexp.MustCompile(`(?i)^(www\.|m\.|music\.)?(youtube\.com|youtu\.be)$`)
	ytWatch = regexp.MustCompile(`(?i)(?:v=|/embed/|/shorts/|/live/|/v/)([A-Za-z0-9_-]{6,})`)
	ytBe    = regexp.MustCompile(`(?i)^/([A-Za-z0-9_-]{6,})`)
)

// YouTube resolves watch URLs via Data API v3 or oEmbed fallback.
type YouTube struct {
	client *http.Client
	ua     string
	apiKey string
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

// Resolve prefers Data API when a key is set; otherwise oEmbed title+author.
func (y *YouTube) Resolve(ctx context.Context, u *url.URL) (string, bool, error) {
	id := youtubeID(u)
	if id == "" {
		return "", false, nil
	}
	if strings.TrimSpace(y.apiKey) != "" {
		return y.resolveAPI(ctx, id)
	}
	if y.log != nil {
		y.log.Printf("youtube: no api_key — oEmbed only (duration/likes need Data API)")
	}
	return y.resolveOEmbed(ctx, id)
}

type ytAPIResp struct {
	Items []struct {
		Snippet struct {
			Title        string `json:"title"`
			ChannelTitle string `json:"channelTitle"`
			PublishedAt  string `json:"publishedAt"`
		} `json:"snippet"`
		ContentDetails struct {
			Duration string `json:"duration"`
		} `json:"contentDetails"`
		Statistics struct {
			LikeCount string `json:"likeCount"`
			ViewCount string `json:"viewCount"`
		} `json:"statistics"`
	} `json:"items"`
}

func (y *YouTube) resolveAPI(ctx context.Context, id string) (string, bool, error) {
	q := url.Values{}
	q.Set("part", "snippet,contentDetails,statistics")
	q.Set("id", id)
	q.Set("key", y.apiKey)
	apiURL := "https://www.googleapis.com/youtube/v3/videos?" + q.Encode()

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
	var parsed ytAPIResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", false, err
	}
	if len(parsed.Items) == 0 {
		return "", false, nil
	}
	it := parsed.Items[0]
	dur := formatISODuration(it.ContentDetails.Duration)
	uploaded := it.Snippet.PublishedAt
	if t, err := time.Parse(time.RFC3339, it.Snippet.PublishedAt); err == nil {
		uploaded = t.UTC().Format("2006-01-02")
	}
	// Official API no longer returns dislikes — likes only.
	return fmt.Sprintf("YouTube: %s | %s | %s | uploaded %s | likes %s",
		it.Snippet.Title, it.Snippet.ChannelTitle, dur, uploaded, it.Statistics.LikeCount), true, nil
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
	return fmt.Sprintf("YouTube: %s | %s", parsed.Title, parsed.AuthorName), true, nil
}

// formatISODuration turns PT1H2M3S into 1h2m3s.
func formatISODuration(iso string) string {
	iso = strings.TrimPrefix(iso, "PT")
	if iso == "" {
		return "?"
	}
	var out strings.Builder
	n := ""
	for _, r := range iso {
		if r >= '0' && r <= '9' {
			n += string(r)
			continue
		}
		if n == "" {
			continue
		}
		switch r {
		case 'H':
			out.WriteString(n + "h")
		case 'M':
			out.WriteString(n + "m")
		case 'S':
			out.WriteString(n + "s")
		}
		n = ""
	}
	if out.Len() == 0 {
		return iso
	}
	return out.String()
}
