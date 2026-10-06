package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

const (
	configFileName = "config.yaml"
	stateFileName  = "state.json"
)

// Schema versions of the files this build reads and writes. A file with any
// other version is refused rather than rewritten, so a newer release's data
// is never clobbered.
const (
	configVersion = 1
	stateVersion  = 1
)

// defaultTags are written on first run. Their IDs never change, so a rename
// keeps every assignment.
var defaultTags = []domain.Tag{
	{ID: "in-progress", Name: "In Progress", Color: "#3B82F6"},
	{ID: "review", Name: "Review", Color: "#A855F7"},
	{ID: "testing", Name: "Testing", Color: "#F59E0B"},
	{ID: "demo", Name: "Demo", Color: "#10B981"},
	{ID: "done", Name: "Done", Color: "#6B7280", Terminal: true},
}

// Config is the user-editable configuration.
type Config struct {
	// Tags are the lanes after Untagged, in display order.
	Tags []domain.Tag
}

// State is what lanes records on its own about the pull requests it shows.
type State struct {
	// Assignments maps a pull request node ID to the ID of its tag. A pull
	// request with no entry is untagged.
	Assignments map[string]string
}

type configFile struct {
	Version int        `yaml:"version"`
	Tags    []tagEntry `yaml:"tags"`
}

type tagEntry struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	Color    string `yaml:"color"`
	Terminal bool   `yaml:"terminal,omitempty"`
}

type stateFile struct {
	Version     int               `json:"version"`
	Assignments map[string]string `json:"assignments"`
}

// LoadConfig reads the config file in dir. On first run, when there is no
// file yet, it writes one with the default tags and returns that.
func LoadConfig(dir string) (Config, error) {
	path := filepath.Join(dir, configFileName)
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return writeDefaultConfig(path)
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var file configFile
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	if file.Version != configVersion {
		return Config{}, fmt.Errorf("read config %s: unsupported version %d (want %d)", path, file.Version, configVersion)
	}
	tags, err := tagsFromFile(file.Tags)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	return Config{Tags: tags}, nil
}

func writeDefaultConfig(path string) (Config, error) {
	file := configFile{Version: configVersion}
	for _, t := range defaultTags {
		file.Tags = append(file.Tags, tagEntry(t))
	}
	raw, err := yaml.Marshal(file)
	if err != nil {
		return Config{}, fmt.Errorf("encode default config: %w", err)
	}
	if err := writeAtomic(path, raw); err != nil {
		return Config{}, fmt.Errorf("write default config: %w", err)
	}
	return Config{Tags: append([]domain.Tag(nil), defaultTags...)}, nil
}

func tagsFromFile(entries []tagEntry) ([]domain.Tag, error) {
	seen := map[string]bool{}
	tags := make([]domain.Tag, 0, len(entries))
	for i, e := range entries {
		switch {
		case e.ID == "":
			return nil, fmt.Errorf("tag %d has no id", i+1)
		case seen[e.ID]:
			return nil, fmt.Errorf("tag id %q is used more than once", e.ID)
		case e.Name == "":
			return nil, fmt.Errorf("tag %q has no name", e.ID)
		}
		seen[e.ID] = true
		tags = append(tags, domain.Tag(e))
	}
	return tags, nil
}

// LoadState reads the state file in dir. A missing file is an empty state.
func LoadState(dir string) (State, error) {
	path := filepath.Join(dir, stateFileName)
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return State{Assignments: map[string]string{}}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("read state: %w", err)
	}
	var file stateFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return State{}, fmt.Errorf("read state %s: %w", path, err)
	}
	if file.Version != stateVersion {
		return State{}, fmt.Errorf("read state %s: unsupported version %d (want %d)", path, file.Version, stateVersion)
	}
	if file.Assignments == nil {
		file.Assignments = map[string]string{}
	}
	return State{Assignments: file.Assignments}, nil
}

// SaveState writes the state file in dir atomically.
func SaveState(dir string, s State) error {
	assignments := s.Assignments
	if assignments == nil {
		assignments = map[string]string{}
	}
	raw, err := json.MarshalIndent(stateFile{Version: stateVersion, Assignments: assignments}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	if err := writeAtomic(filepath.Join(dir, stateFileName), append(raw, '\n')); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}

// writeAtomic replaces path with data so that a crash leaves either the old
// file or the new one, never a partial write: the temp file sits in the same
// directory because a rename is only atomic within one filesystem.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	committed = true
	return nil
}
