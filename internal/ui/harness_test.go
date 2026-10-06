package ui_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/jehielmartinez/gh-lanes/internal/github"
	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
	"github.com/jehielmartinez/gh-lanes/internal/store"
	"github.com/jehielmartinez/gh-lanes/internal/ui"
)

const (
	placeholderToken = "placeholder-token"
	waitTimeout      = 3 * time.Second
)

// fixedZone keeps local-time output identical on every machine.
var fixedZone = time.FixedZone("UTC-5", -5*60*60)

// harness is the one test seam: the real root model under teatest, the fake
// GraphQL transport, a temporary config directory and a controllable clock.
type harness struct {
	t         *testing.T
	tm        *teatest.TestModel
	transport *githubtest.Transport
	configDir string
	clock     *clock
	screen    *screen
}

type harnessOption func(*harnessConfig)

type harnessConfig struct {
	env    map[string]string
	config string
}

// withEnv sets an environment variable before the GitHub layer resolves auth.
func withEnv(key, value string) harnessOption {
	return func(c *harnessConfig) { c.env[key] = value }
}

// withConfig seeds the config file before the app starts.
func withConfig(yaml string) harnessOption {
	return func(c *harnessConfig) { c.config = yaml }
}

// newHarness starts the app. The transport must have its replies queued before
// the call, since the app fetches as soon as it starts.
func newHarness(t *testing.T, transport *githubtest.Transport, opts ...harnessOption) *harness {
	t.Helper()
	cfg := harnessConfig{env: map[string]string{}}
	for _, o := range opts {
		o(&cfg)
	}
	isolateGHAuth(t)
	t.Setenv("GH_TOKEN", placeholderToken)
	for k, v := range cfg.env {
		t.Setenv(k, v)
	}

	client, err := github.New(transport)
	if err != nil {
		t.Fatalf("github.New: %v", err)
	}
	configDir := t.TempDir()
	if cfg.config != "" {
		if err := os.WriteFile(filepath.Join(configDir, store.ConfigFile), []byte(cfg.config), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	h := &harness{
		t:         t,
		transport: transport,
		configDir: configDir,
		clock:     &clock{now: time.Date(2026, 3, 5, 12, 0, 0, 0, fixedZone)},
		screen:    &screen{},
	}
	root := ui.New(ui.Options{GitHub: client, ConfigDir: h.configDir, Now: h.clock.Now, After: h.clock.After})
	h.tm = teatest.NewTestModel(t, spy{inner: root, screen: h.screen}, teatest.WithInitialTermSize(100, 30))
	t.Cleanup(func() { _ = h.tm.Quit() })
	return h
}

// isolateGHAuth stops go-gh from finding the developer's or CI's real login:
// an empty gh config dir, no token variables and no gh binary to ask.
func isolateGHAuth(t *testing.T) {
	t.Helper()
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_PATH", filepath.Join(t.TempDir(), "no-gh"))
	for _, k := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN", "GH_HOST"} {
		t.Setenv(k, "")
	}
}

func fixture(name string) string {
	return filepath.Join("..", "..", "testdata", "graphql", name)
}

func (h *harness) press(key string) {
	h.t.Helper()
	switch key {
	case "ctrl+c":
		h.tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	default:
		h.tm.Type(key)
	}
}

// waitForScreen waits until the rendered screen, with styling removed,
// satisfies cond, and returns it.
func (h *harness) waitForScreen(desc string, cond func(screen string) bool) string {
	h.t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for {
		plain := h.screen.plain()
		if cond(plain) {
			return plain
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("timed out waiting for %s; screen:\n%s", desc, plain)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (h *harness) waitForText(text string) string {
	h.t.Helper()
	return h.waitForScreen(text, func(s string) bool { return strings.Contains(s, text) })
}

func (h *harness) waitFinished() {
	h.t.Helper()
	h.tm.WaitFinished(h.t, teatest.WithFinalTimeout(waitTimeout))
}

// spy wraps the root model only to observe what it draws; every message and
// command passes through untouched.
type spy struct {
	inner  tea.Model
	screen *screen
}

func (s spy) Init() tea.Cmd { return s.inner.Init() }

func (s spy) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := s.inner.Update(msg)
	return spy{inner: next, screen: s.screen}, cmd
}

func (s spy) View() tea.View {
	v := s.inner.View()
	s.screen.set(v.Content)
	return v
}

type screen struct {
	mu  sync.Mutex
	raw string
}

func (s *screen) set(raw string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.raw = raw
}

func (s *screen) content() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.raw
}

var styling = regexp.MustCompile(`\x1b\[[0-9;:?]*[ -/]*[@-~]`)

func (s *screen) plain() string {
	lines := strings.Split(styling.ReplaceAllString(s.content(), ""), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n")
}

// advance moves the clock forward once the app is waiting on it, so a tick
// can't be missed by arriving before the app asked for it.
func (h *harness) advance(d time.Duration) {
	h.t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for !h.clock.waiting() {
		if time.Now().After(deadline) {
			h.t.Fatalf("timed out waiting for the app to wait on the clock")
		}
		time.Sleep(time.Millisecond)
	}
	h.clock.Advance(d)
}

// settle advances the clock a second at a time, letting the app handle each
// tick, so everything due in d has happened by the time it returns.
func (h *harness) settle(d time.Duration) {
	h.t.Helper()
	for range int(d / time.Second) {
		h.advance(time.Second)
	}
}

// waitForRequests waits until the transport has received n requests.
func (h *harness) waitForRequests(n int) []githubtest.Request {
	h.t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for {
		reqs := h.transport.Requests()
		if len(reqs) >= n {
			return reqs
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("timed out waiting for %d requests, got %d", n, len(reqs))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// clock is the controllable clock: time moves only when a test advances it,
// and After fires once it has moved far enough.
type clock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []waiter
}

type waiter struct {
	at time.Time
	ch chan time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan time.Time, 1)
	if d <= 0 {
		ch <- c.now
		return ch
	}
	c.waiters = append(c.waiters, waiter{at: c.now.Add(d), ch: ch})
	return ch
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	pending := c.waiters[:0]
	for _, w := range c.waiters {
		if w.at.After(c.now) {
			pending = append(pending, w)
			continue
		}
		w.ch <- c.now
	}
	c.waiters = pending
}

func (c *clock) waiting() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.waiters) > 0
}
