package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigFile is the name of the user-editable config file in the config
// directory.
const ConfigFile = "config.yaml"

// DefaultRefreshInterval is how often the board refreshes when the config
// doesn't say.
const DefaultRefreshInterval = 60 * time.Second

// MinRefreshInterval is the shortest refresh interval accepted, so a typo
// can't spend the whole rate-limit budget.
const MinRefreshInterval = 10 * time.Second

// Config is the user's settings.
type Config struct {
	RefreshInterval time.Duration
}

type configFile struct {
	RefreshInterval string `yaml:"refresh_interval"`
}

// LoadConfig reads the config file in dir. A missing file, or a setting it
// leaves out, means the default. When the file can't be used, LoadConfig
// returns the defaults together with the error, so the app can run on and
// say what was wrong.
func LoadConfig(dir string) (Config, error) {
	cfg := Config{RefreshInterval: DefaultRefreshInterval}
	path := filepath.Join(dir, ConfigFile)
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	var file configFile
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	if file.RefreshInterval != "" {
		d, err := time.ParseDuration(file.RefreshInterval)
		if err != nil || d < MinRefreshInterval {
			return cfg, fmt.Errorf("%s: refresh_interval %q must be a duration of at least %s, like 60s or 5m; using %s",
				ConfigFile, file.RefreshInterval, MinRefreshInterval, DefaultRefreshInterval)
		}
		cfg.RefreshInterval = d
	}
	return cfg, nil
}
