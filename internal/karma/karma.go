// Package karma stores word/phrase scores in SQLite beside the config file.
package karma

import (
	"database/sql"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

// Mode is how a bump was rolled (controls channel reply).
type Mode int

const (
	ModeSilent Mode = iota // ++ / --
	ModeDice               // +d / -d
	ModeRandom             // +N..M / -N..M
)

// MaxRandomBound is the highest allowed end of a +N..M range.
const MaxRandomBound = 23

// Bump is one parsed karma adjustment (or a rejection).
type Bump struct {
	Phrase string
	Delta  int  // signed change to apply
	Result int  // positive magnitude of the roll / step
	Mode   Mode // reply style
	Reject string
}

// Store is a SQLite-backed phrase → score map.
type Store struct {
	db   *sql.DB
	path string
}

// PathFor builds karma-{nick}-{host}.db beside the config file.
func PathFor(configPath, nick, host string) string {
	dir := filepath.Dir(configPath)
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, fmt.Sprintf("karma-%s-%s.db", nick, host))
}

// Open creates or opens the karma DB at path.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// One writer; avoid busy surprises on reload reopen.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS karma (
		phrase TEXT PRIMARY KEY NOT NULL,
		score INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db, path: path}, nil
}

// Close releases the DB handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Get returns the score for phrase. found=false when the row is missing (score 0).
func (s *Store) Get(phrase string) (score int, found bool, err error) {
	err = s.db.QueryRow(`SELECT score FROM karma WHERE phrase = ?`, phrase).Scan(&score)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return score, true, nil
}

// Add applies delta and returns the score before and after the write.
func (s *Store) Add(phrase string, delta int) (from, to int, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	err = tx.QueryRow(`SELECT score FROM karma WHERE phrase = ?`, phrase).Scan(&from)
	if err == sql.ErrNoRows {
		from = 0
		to = delta
		_, err = tx.Exec(`INSERT INTO karma(phrase, score) VALUES(?, ?)`, phrase, to)
	} else if err != nil {
		return 0, 0, err
	} else {
		to = from + delta
		_, err = tx.Exec(`UPDATE karma SET score = ? WHERE phrase = ?`, to, phrase)
	}
	if err != nil {
		return 0, 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return from, to, nil
}

var (
	// +N..M / -N..M (no "r"); checked before ++/-- so foo+2..6 is random, foo++ stays ±1.
	reRandom = regexp.MustCompile(`^(.+?)([+-])(\d+)\.\.(\d+)`)
	reDice   = regexp.MustCompile(`(?i)^(.+?)([+-])d(?:\s|$)`)
	rePlain  = regexp.MustCompile(`^(.+?)(\+\+|--)`)
)

var partners = []string{
	"partner", "homie", "hermano", "muchacho", "bruv", "bro",
	"dude", "mfer", "stank-ass", "dipshit", "wappie",
}

// ParseBump finds a karma operator at the start of the line.
// ok=false means no karma syntax. Reject is set when the range is too large.
func ParseBump(text string) (Bump, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Bump{}, false
	}

	// Order: +N..M, then ++/-- (before +d so "food++" is not a dice bump).
	if m := reRandom.FindStringSubmatch(text); m != nil {
		phrase := strings.TrimSpace(m[1])
		if phrase == "" {
			return Bump{}, false
		}
		lo, _ := strconv.Atoi(m[3])
		hi, _ := strconv.Atoi(m[4])
		if lo > MaxRandomBound || hi > MaxRandomBound {
			return Bump{Phrase: phrase, Mode: ModeRandom, Reject: RejectMessage()}, true
		}
		if lo > hi {
			lo, hi = hi, lo
		}
		result := lo + rand.IntN(hi-lo+1)
		delta := result
		if m[2] == "-" {
			delta = -result
		}
		return Bump{Phrase: phrase, Delta: delta, Result: result, Mode: ModeRandom}, true
	}

	if m := rePlain.FindStringSubmatch(text); m != nil {
		phrase := strings.TrimSpace(m[1])
		if phrase == "" {
			return Bump{}, false
		}
		delta := 1
		if m[2] == "--" {
			delta = -1
		}
		return Bump{Phrase: phrase, Delta: delta, Result: 1, Mode: ModeSilent}, true
	}

	if m := reDice.FindStringSubmatch(text); m != nil {
		phrase := strings.TrimSpace(m[1])
		if phrase == "" {
			return Bump{}, false
		}
		result := 1 + rand.IntN(6)
		delta := result
		if m[2] == "-" {
			delta = -result
		}
		return Bump{Phrase: phrase, Delta: delta, Result: result, Mode: ModeDice}, true
	}

	return Bump{}, false
}

// RejectMessage is the "range too high" insult with a random partner word.
func RejectMessage() string {
	p := partners[rand.IntN(len(partners))]
	return fmt.Sprintf("woah there %s that's way too much karma there! hakuna yer tata's", p)
}

// AdjustReply formats the channel line for dice/random bumps; empty for silent.
func AdjustReply(mode Mode, result, from, to int) string {
	switch mode {
	case ModeDice:
		return fmt.Sprintf("karma adjusted with dice roll %d, new karma: %d -> %d", result, from, to)
	case ModeRandom:
		return fmt.Sprintf("karma adjusted with random %d, new karma: %d -> %d", result, from, to)
	default:
		return ""
	}
}

// HelpText is the bare .karma explainer.
func HelpText() string {
	return "karma: phrase++ / phrase-- (±1, silent); phrase+d / phrase-d (dice 1-6); phrase+N..M / phrase-N..M (random, max 23). .karma <phrase> looks up a score."
}
