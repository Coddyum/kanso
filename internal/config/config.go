// Package config is the small file that says where the user's folder is and
// which port the app listens on. `kanso init` writes it, the Settings screen
// changes it, and the server, the Mac app and every `kanso` command read it:
// ~/Library/Application Support/Kanso/config.json.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/coddyum/kanso/internal/brand"
)

// DefaultPort is the port of a new install.
const DefaultPort = 48219

// Config is the content of the file.
type Config struct {
	Root string `json:"root"` // the user's folder, absolute; "" before `kanso init`
	Port int    `json:"port"`
	Demo bool   `json:"demo,omitempty"` // the app runs on the demo folder; Root is kept for when it stops
}

// Dir is the app's own folder in Application Support.
func Dir(home string) string {
	return filepath.Join(home, "Library", "Application Support", brand.Name)
}

// Path is the file itself.
func Path(home string) string { return filepath.Join(Dir(home), "config.json") }

// Load reads the file; a missing one is the default (no folder yet, the default port).
func Load(home string) (Config, error) {
	c := Config{Port: DefaultPort}
	data, err := os.ReadFile(Path(home))
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, fmt.Errorf("config: %w", err)
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("config: %s: %w", Path(home), err)
	}
	if c.Port <= 0 || c.Port > 65535 {
		c.Port = DefaultPort
	}
	return c, nil
}

// Save writes the file atomically: a temporary file, then a rename.
func Save(home string, c Config) error {
	if !filepath.IsAbs(c.Root) {
		return fmt.Errorf("config: the folder must be an absolute path: %q", c.Root)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(Dir(home), 0o755); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	tmp := Path(home) + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return os.Rename(tmp, Path(home))
}
