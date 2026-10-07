package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// ConfigFile is the name of the user-editable config file in the config
// directory.
const ConfigFile = "config.yaml"

const stateFileName = "state.json"

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
	{ID: "in-progress", Name: "In Progress", Color: domain.TagColors[0]},
	{ID: "review", Name: "Review", Color: domain.TagColors[1]},
	{ID: "testing", Name: "Testing", Color: domain.TagColors[2]},
	{ID: "demo", Name: "Demo", Color: domain.TagColors[3]},
	{ID: "done", Name: "Done", Color: domain.TagColors[4], Terminal: true},
}

// Config is the user-editable configuration.
type Config struct {
	// Tags are the lanes after Untagged, in display order.
	Tags []domain.Tag
	// RefreshInterval is how often the board refreshes.
	RefreshInterval time.Duration
	// Filter hides repositories' pull requests. A config without one hides
	// nothing.
	Filter domain.Filter
	// Grouping clusters cards into groups. A config without one, or with a
	// value this build doesn't know, groups nothing.
	Grouping domain.Grouping
	// TitlePattern is matched against titles when grouping by title pattern.
	// It is never nil: a config without one, or with one that doesn't
	// compile, has the default.
	TitlePattern *regexp.Regexp
}

// State is what lanes records on its own about the pull requests it shows.
type State struct {
	// Assignments maps a pull request node ID to the ID of its tag. A pull
	// request with no entry is untagged.
	Assignments map[string]string
	// Archived lists the pull requests archived off the board.
	Archived []domain.Archived
	// Snapshots maps a pull request node ID to what it looked like when last
	// seen. It is nil until the first board load has recorded what was on the
	// board then, which is how a later run tells new pull requests apart.
	Snapshots map[string]domain.Snapshot
}

type configFile struct {
	Version         int        `yaml:"version"`
	RefreshInterval string     `yaml:"refresh_interval,omitempty"`
	Tags            []tagEntry `yaml:"tags"`
	// Filter is a pointer so a file without one is saved without one.
	Filter *filterEntry `yaml:"filter,omitempty"`
	// Grouping is kept as written, so saving never rewrites a value this
	// build doesn't know.
	Grouping string `yaml:"grouping,omitempty"`
	// TitlePattern is kept as written too, so a pattern that doesn't compile
	// is still there to fix by hand.
	TitlePattern string `yaml:"title_pattern,omitempty"`
}

type filterEntry struct {
	ExcludedOwners []string          `yaml:"excluded_owners,omitempty"`
	Repositories   map[string]string `yaml:"repositories,omitempty"`
}

type tagEntry struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	Color    string `yaml:"color"`
	Terminal bool   `yaml:"terminal,omitempty"`
}

type stateFile struct {
	Version     int                      `json:"version"`
	Assignments map[string]string        `json:"assignments"`
	Archived    []archivedEntry          `json:"archived"`
	Snapshots   map[string]snapshotEntry `json:"snapshots"`
}

type archivedEntry struct {
	ID     string        `json:"id"`
	Open   bool          `json:"open"`
	Tag    string        `json:"tag,omitempty"`
	Origin domain.Origin `json:"origin,omitempty"`
}

type snapshotEntry struct {
	SeenAt         time.Time             `json:"seen_at"`
	Checks         domain.CheckState     `json:"checks"`
	Mergeable      domain.Mergeable      `json:"mergeable"`
	ReviewDecision domain.ReviewDecision `json:"review_decision"`
	Comments       int                   `json:"comments"`
	Reviews        int                   `json:"reviews"`
	Draft          bool                  `json:"draft"`
	State          domain.State          `json:"state"`
}

// LoadConfig reads the config file in dir. On first run, when there is no
// file yet, it writes one with the default tags and returns that. A setting
// the file leaves out means the default. Whenever it returns an error the
// config still carries a usable refresh interval and title pattern; an error
// Usable accepts leaves the rest of the config intact, any other error means
// the file couldn't be used.
func LoadConfig(dir string) (Config, error) {
	failed := Config{RefreshInterval: DefaultRefreshInterval, TitlePattern: defaultTitlePattern}
	path := filepath.Join(dir, ConfigFile)
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return writeDefaultConfig(path)
	}
	if err != nil {
		return failed, fmt.Errorf("read config: %w", err)
	}
	var file configFile
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return failed, fmt.Errorf("read config %s: %w", path, err)
	}
	if file.Version != configVersion {
		return failed, fmt.Errorf("read config %s: unsupported version %d (want %d)", path, file.Version, configVersion)
	}
	tags, err := tagsFromFile(file.Tags)
	if err != nil {
		return failed, fmt.Errorf("read config %s: %w", path, err)
	}
	filter, err := filterFromFile(file.Filter)
	if err != nil {
		return failed, fmt.Errorf("read config %s: %w", path, err)
	}
	interval, intervalErr := parseRefreshInterval(file.RefreshInterval)
	pattern, patternErr := parseTitlePattern(file.TitlePattern)
	cfg := Config{Tags: tags, RefreshInterval: interval, Filter: filter, Grouping: groupingFromFile(file.Grouping), TitlePattern: pattern}
	return cfg, errors.Join(intervalErr, patternErr)
}

func groupingFromFile(value string) domain.Grouping {
	switch g := domain.Grouping(value); g {
	case domain.GroupingOwner, domain.GroupingRepository, domain.GroupingTitlePattern:
		return g
	}
	return domain.GroupingNone
}

func filterFromFile(entry *filterEntry) (domain.Filter, error) {
	if entry == nil {
		return domain.Filter{}, nil
	}
	f := domain.Filter{ExcludedOwners: entry.ExcludedOwners, Repositories: map[string]domain.RepoChoice{}}
	seen := map[string]string{}
	for repo, choice := range entry.Repositories {
		switch c := domain.RepoChoice(choice); c {
		case domain.RepoIncluded, domain.RepoExcluded:
			f.Repositories[repo] = c
		default:
			return domain.Filter{}, fmt.Errorf("filter: repository %q is %q, want %q or %q", repo, choice, domain.RepoIncluded, domain.RepoExcluded)
		}
		// Names match case-insensitively, so two spellings of one repository
		// would leave its choice to map order.
		key := strings.ToLower(repo)
		if other, dup := seen[key]; dup {
			return domain.Filter{}, fmt.Errorf("filter: repositories %q and %q are the same repository", min(other, repo), max(other, repo))
		}
		seen[key] = repo
	}
	return f, nil
}

func filterToFile(f domain.Filter) *filterEntry {
	if len(f.ExcludedOwners) == 0 && len(f.Repositories) == 0 {
		return nil
	}
	entry := &filterEntry{ExcludedOwners: f.ExcludedOwners}
	if len(f.Repositories) > 0 {
		entry.Repositories = make(map[string]string, len(f.Repositories))
		for repo, choice := range f.Repositories {
			entry.Repositories[repo] = string(choice)
		}
	}
	return entry
}

func writeDefaultConfig(path string) (Config, error) {
	file := configFile{Version: configVersion}
	for _, t := range defaultTags {
		file.Tags = append(file.Tags, tagEntry(t))
	}
	raw, err := yaml.Marshal(file)
	if err != nil {
		return Config{RefreshInterval: DefaultRefreshInterval, TitlePattern: defaultTitlePattern}, fmt.Errorf("encode default config: %w", err)
	}
	if err := writeAtomic(path, raw); err != nil {
		return Config{RefreshInterval: DefaultRefreshInterval, TitlePattern: defaultTitlePattern}, fmt.Errorf("write default config: %w", err)
	}
	return Config{Tags: append([]domain.Tag(nil), defaultTags...), RefreshInterval: DefaultRefreshInterval, TitlePattern: defaultTitlePattern}, nil
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

// SaveConfig replaces the tags and the filter in the config file in dir,
// atomically, and keeps every other setting as the file has it. A filter that
// hides nothing is left out of the file. It refuses to touch a file it can't
// read, so a hand edit it doesn't understand is never overwritten.
func SaveConfig(dir string, tags []domain.Tag, filter domain.Filter) error {
	path := filepath.Join(dir, ConfigFile)
	file := configFile{Version: configVersion}
	raw, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return fmt.Errorf("read config: %w", err)
	default:
		if err := yaml.Unmarshal(raw, &file); err != nil {
			return fmt.Errorf("read config %s: %w", path, err)
		}
		if file.Version != configVersion {
			return fmt.Errorf("read config %s: unsupported version %d (want %d)", path, file.Version, configVersion)
		}
	}
	file.Tags = make([]tagEntry, len(tags))
	for i, t := range tags {
		file.Tags[i] = tagEntry(t)
	}
	if _, err := tagsFromFile(file.Tags); err != nil {
		return fmt.Errorf("save tags: %w", err)
	}
	file.Filter = filterToFile(filter)
	out, err := yaml.Marshal(file)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := writeAtomic(path, out); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
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
	var archived []domain.Archived
	for _, e := range file.Archived {
		if e.ID != "" {
			archived = append(archived, domain.Archived(e))
		}
	}
	st := State{Assignments: file.Assignments, Archived: archived}
	if file.Snapshots != nil {
		st.Snapshots = make(map[string]domain.Snapshot, len(file.Snapshots))
		for id, e := range file.Snapshots {
			st.Snapshots[id] = domain.Snapshot(e)
		}
	}
	return st, nil
}

// SaveState writes the state file in dir atomically.
func SaveState(dir string, s State) error {
	assignments := s.Assignments
	if assignments == nil {
		assignments = map[string]string{}
	}
	archived := make([]archivedEntry, 0, len(s.Archived))
	for _, a := range s.Archived {
		archived = append(archived, archivedEntry(a))
	}
	file := stateFile{Version: stateVersion, Assignments: assignments, Archived: archived}
	if s.Snapshots != nil {
		file.Snapshots = make(map[string]snapshotEntry, len(s.Snapshots))
		for id, snap := range s.Snapshots {
			file.Snapshots[id] = snapshotEntry(snap)
		}
	}
	raw, err := json.MarshalIndent(file, "", "  ")
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
