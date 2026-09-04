package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConfigNameSuffix matches any basename ending in t3b.conf (e.g. bot.t3b.conf).
const ConfigNameSuffix = "t3b.conf"

// ErrNoConfig means Discover found no *t3b.conf files in dir.
var ErrNoConfig = errors.New("no t3b config found")

// ErrManyConfigs means Discover found more than one *t3b.conf in dir.
type ErrManyConfigs struct {
	Dir   string
	Names []string
}

func (e *ErrManyConfigs) Error() string {
	return fmt.Sprintf(
		"too many configs ending in %q in %s (%s) — pick one with -config, or clean up your freaking mess",
		ConfigNameSuffix, e.Dir, strings.Join(e.Names, ", "),
	)
}

// Discover finds exactly one file in dir whose name ends with ConfigNameSuffix.
// Returns ErrNoConfig or *ErrManyConfigs when the count is not one.
func Discover(dir string) (string, error) {
	if dir == "" {
		dir = "."
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("scan config dir %q: %w", dir, err)
	}

	var matches []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ConfigNameSuffix) {
			matches = append(matches, name)
		}
	}

	switch len(matches) {
	case 0:
		return "", ErrNoConfig
	case 1:
		return filepath.Join(dir, matches[0]), nil
	default:
		return "", &ErrManyConfigs{Dir: dir, Names: matches}
	}
}
