// Package linklog persists resolved channel URLs as JSONL for Meat Bag search.
package linklog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// PageSize is how many links one IRC page shows before .more.
const PageSize = 3

// Entry is one resolved link line in the log file.
type Entry struct {
	ID       int    `json:"id"`
	Datetime string `json:"datetime"`
	Channel  string `json:"channel"`
	User     string `json:"user"`
	Domain   string `json:"domain"`
	URL      string `json:"URL"`
	Title    string `json:"title"`
}

// Stats is aggregate info for bare .link.
type Stats struct {
	Total   int
	Domains int
}

// Store is an append-only JSONL link log with an in-memory index for search.
type Store struct {
	mu      sync.Mutex
	path    string
	f       *os.File
	nextID  int
	entries []Entry
}

// PathFor builds links-{nick}-{host}.log beside the config file.
func PathFor(configPath, nick, host string) string {
	dir := filepath.Dir(configPath)
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, fmt.Sprintf("links-%s-%s.log", nick, host))
}

// DomainOf returns the hostname for a URL string (empty if unparseable).
func DomainOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// Open creates or opens a JSONL store at path and loads existing entries.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	s := &Store{path: path, f: f, nextID: 1}
	if err := s.loadLocked(); err != nil {
		_ = f.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) loadLocked() error {
	if _, err := s.f.Seek(0, 0); err != nil {
		return err
	}
	sc := bufio.NewScanner(s.f)
	// Long IRC titles / URLs — allow larger lines than default 64KiB.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	var entries []Entry
	maxID := 0
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return fmt.Errorf("linklog %s:%d: %w", s.path, lineNo, err)
		}
		entries = append(entries, e)
		if e.ID > maxID {
			maxID = e.ID
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	s.entries = entries
	s.nextID = maxID + 1
	if s.nextID < 1 {
		s.nextID = 1
	}
	// Position for appends.
	if _, err := s.f.Seek(0, 2); err != nil {
		return err
	}
	return nil
}

// Path returns the log file path.
func (s *Store) Path() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.path
}

// Close flushes and closes the underlying file.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f == nil {
		return nil
	}
	err := s.f.Close()
	s.f = nil
	return err
}

// Append writes a new entry (assigns id + datetime). Input Channel/User/URL/Title required.
func (s *Store) Append(channel, user, rawURL, title string) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f == nil {
		return Entry{}, fmt.Errorf("linklog: closed")
	}
	e := Entry{
		ID:       s.nextID,
		Datetime: time.Now().Format(time.RFC3339),
		Channel:  channel,
		User:     user,
		Domain:   DomainOf(rawURL),
		URL:      rawURL,
		Title:    title,
	}
	b, err := json.Marshal(e)
	if err != nil {
		return Entry{}, err
	}
	if _, err := s.f.Write(append(b, '\n')); err != nil {
		return Entry{}, err
	}
	if err := s.f.Sync(); err != nil {
		return Entry{}, err
	}
	s.entries = append(s.entries, e)
	s.nextID++
	return e, nil
}

// Stats returns total entries and unique domains.
func (s *Store) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := make(map[string]struct{})
	for _, e := range s.entries {
		if e.Domain != "" {
			seen[e.Domain] = struct{}{}
		}
	}
	return Stats{Total: len(s.entries), Domains: len(seen)}
}

// ByID returns the entry with the given id.
func (s *Store) ByID(id int) (Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.entries {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// Last returns up to n newest entries (newest first). n<=0 yields empty.
func (s *Store) Last(n int) []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n <= 0 || len(s.entries) == 0 {
		return nil
	}
	if n > len(s.entries) {
		n = len(s.entries)
	}
	out := make([]Entry, n)
	for i := 0; i < n; i++ {
		out[i] = s.entries[len(s.entries)-1-i]
	}
	return out
}

// Search returns entries whose domain, URL, or title contain q (case-insensitive), ascending by id.
func (s *Store) Search(q string) []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	var out []Entry
	for _, e := range s.entries {
		if strings.Contains(strings.ToLower(e.Domain), q) ||
			strings.Contains(strings.ToLower(e.URL), q) ||
			strings.Contains(strings.ToLower(e.Title), q) {
			out = append(out, e)
		}
	}
	return out
}

// FormatEntry is one IRC-friendly line for a logged link.
func FormatEntry(e Entry) string {
	return fmt.Sprintf("#%d [%s] <%s> %s — %s", e.ID, e.Channel, e.User, e.URL, e.Title)
}

// FormatPage returns up to PageSize formatted lines from entries[offset:], plus an optional footer.
// next is the offset for a following .more; hasMore is true when more remain.
func FormatPage(entries []Entry, offset int) (lines []string, next int, hasMore bool) {
	total := len(entries)
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return nil, offset, false
	}
	end := offset + PageSize
	if end > total {
		end = total
	}
	for i := offset; i < end; i++ {
		lines = append(lines, FormatEntry(entries[i]))
	}
	from := offset + 1
	to := end
	footer := fmt.Sprintf("Showing %d-%d of %d.", from, to, total)
	if end < total {
		footer += " Send .more or .m for next."
		hasMore = true
	}
	lines = append(lines, footer)
	return lines, end, hasMore
}
