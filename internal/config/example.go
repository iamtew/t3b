package config

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
)

//go:embed t3b.conf.example
var exampleConf []byte

// ErrExampleExists means WriteExample refused to clobber an existing file.
var ErrExampleExists = errors.New("config file already exists")

// WriteExample writes the embedded example config to path (O_EXCL — no overwrite).
func WriteExample(path string) error {
	if path == "" {
		path = DefaultPath
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%w: %q", ErrExampleExists, path)
		}
		return fmt.Errorf("write example config %q: %w", path, err)
	}
	defer f.Close()
	if _, err := f.Write(exampleConf); err != nil {
		return fmt.Errorf("write example config %q: %w", path, err)
	}
	return nil
}
