// Package config defines the shipyard.yaml schema, loads it from disk,
// fills in defaults, and validates it.
//
// The shape mirrors the example in the public docs at
// https://shipyard.philiprehberger.com/docs/config-reference — any change
// here is a public API change.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Default config-file name searched in cwd when a deploy/rollback/etc.
// command runs with no explicit config argument.
const DefaultFilename = "shipyard.yaml"

// Config is the top-level shipyard.yaml document.
type Config struct {
	App         string          `yaml:"app"`
	Host        HostConfig      `yaml:"host"`
	Artifact    ArtifactConfig  `yaml:"artifact"`
	Releases    ReleasesConfig  `yaml:"releases"`
	Shared      SharedConfig    `yaml:"shared"`
	HealthCheck HealthCheck     `yaml:"health_check"`
	Hooks       Hooks           `yaml:"hooks"`
	Lock        LockConfig      `yaml:"lock"`

	// Path that the config was loaded from, set by Load. Useful for relative
	// path resolution (artifact.source, identity_file with ~).
	sourcePath string `yaml:"-"`
}

// HostConfig describes the SSH target.
type HostConfig struct {
	SSH          string `yaml:"ssh"`           // "user@host[:port]"
	IdentityFile string `yaml:"identity_file"` // private key path; ~ expanded
	ReleaseRoot  string `yaml:"release_root"`  // absolute remote path (e.g. /var/www/app)
}

// ArtifactConfig describes the local build artifact to ship.
type ArtifactConfig struct {
	Source string `yaml:"source"` // local path to .zip or .tar.gz
	Format string `yaml:"format"` // "zip" | "tar.gz"; derived from extension if blank
}

// ReleasesConfig describes the auto-prune policy.
type ReleasesConfig struct {
	Keep int `yaml:"keep"` // number of past releases to keep (default 5)
}

// SharedConfig lists files and directories that persist across releases.
// At deploy time these are symlinked into releases/<ts>/ from shared/.
type SharedConfig struct {
	Files []string `yaml:"files"` // e.g. [".env"]
	Dirs  []string `yaml:"dirs"`  // e.g. ["storage", "bootstrap/cache"]
}

// HealthCheck is run after the symlink flip. Failure triggers automatic
// rollback unless --skip-health was passed.
type HealthCheck struct {
	URL     string   `yaml:"url"`
	Expect  string   `yaml:"expect"`  // substring; empty means "any 2xx body"
	Status  int      `yaml:"status"`  // default 200
	Retries int      `yaml:"retries"` // default 10
	Delay   Duration `yaml:"delay"`   // between attempts; default 3s
	Timeout Duration `yaml:"timeout"` // per request; default 5s
}

// Hooks are shell snippets executed at well-defined points in the deploy
// lifecycle. Each entry is passed to `sh -c` on the local or remote box
// per the comments below.
type Hooks struct {
	PreUpload   []string `yaml:"pre_upload"`   // local, before transfer
	PostExtract []string `yaml:"post_extract"` // remote, in releases/<ts>/ before flip
	PostFlip    []string `yaml:"post_flip"`    // remote, after symlink swap
	OnRollback  []string `yaml:"on_rollback"`  // remote, after rollback symlink swap
}

// LockConfig controls the remote mutex that prevents concurrent deploys.
//
// Enabled is a pointer so an unset YAML field (nil) and an explicit
// `enabled: false` (pointer to false) can be distinguished. The default
// when the whole lock block is omitted is enabled.
type LockConfig struct {
	Enabled *bool    `yaml:"enabled"` // default true
	Path    string   `yaml:"path"`    // default <release_root>/shared/.shipyard.lock
	TTL     Duration `yaml:"ttl"`     // stale-lock threshold; default 10m
}

// IsEnabled returns true unless the user explicitly set enabled: false.
func (l LockConfig) IsEnabled() bool {
	if l.Enabled == nil {
		return true
	}
	return *l.Enabled
}

// SourcePath returns the absolute path of the YAML file Load read from,
// or empty if the Config was constructed in memory.
func (c *Config) SourcePath() string {
	return c.sourcePath
}

// Load reads, parses, defaults, and validates a shipyard.yaml.
//
// If path is empty, Load looks for DefaultFilename in the current working
// directory. Relative paths inside the config (artifact.source) are
// resolved against the directory of the loaded file, not the cwd, so
// `shipyard deploy ../other/shipyard.yaml` ships the artifact next to
// that config, not next to the shell's cwd.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultFilename
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving config path: %w", err)
	}

	f, err := os.Open(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("config file not found: %s", abs)
		}
		return nil, fmt.Errorf("opening config %s: %w", abs, err)
	}
	defer f.Close()

	cfg, err := parse(f)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", abs, err)
	}
	cfg.sourcePath = abs

	cfg.applyDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating %s: %w", abs, err)
	}

	return cfg, nil
}

// parse decodes a single YAML document with strict unknown-field rejection.
// Strict mode catches typos like `health-check:` (hyphen vs. underscore)
// that would otherwise silently fall through as defaults.
func parse(r io.Reader) (*Config, error) {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)

	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("config file is empty")
		}
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Artifact.Format == "" && c.Artifact.Source != "" {
		c.Artifact.Format = inferArtifactFormat(c.Artifact.Source)
	}

	if c.Releases.Keep == 0 {
		c.Releases.Keep = 5
	}

	if c.HealthCheck.URL != "" {
		if c.HealthCheck.Status == 0 {
			c.HealthCheck.Status = 200
		}
		if c.HealthCheck.Retries == 0 {
			c.HealthCheck.Retries = 10
		}
		if c.HealthCheck.Delay.Duration() == 0 {
			c.HealthCheck.Delay = Duration(3 * time.Second)
		}
		if c.HealthCheck.Timeout.Duration() == 0 {
			c.HealthCheck.Timeout = Duration(5 * time.Second)
		}
	}

	if c.Lock.IsEnabled() {
		if c.Lock.Path == "" && c.Host.ReleaseRoot != "" {
			c.Lock.Path = c.Host.ReleaseRoot + "/shared/.shipyard.lock"
		}
		if c.Lock.TTL.Duration() == 0 {
			c.Lock.TTL = Duration(10 * time.Minute)
		}
	}
}

func inferArtifactFormat(source string) string {
	lower := strings.ToLower(source)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return "zip"
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return "tar.gz"
	}
	return ""
}

// ValidationError is a list of validation failures keyed by the YAML field
// path that produced them. Returned by Validate as a single error whose
// Error() lists each issue on its own line for human consumption.
type ValidationError struct {
	Issues []FieldIssue
}

// FieldIssue is a single validation problem.
type FieldIssue struct {
	Field   string
	Message string
}

func (v *ValidationError) Error() string {
	if len(v.Issues) == 0 {
		return "validation failed"
	}
	if len(v.Issues) == 1 {
		return fmt.Sprintf("%s: %s", v.Issues[0].Field, v.Issues[0].Message)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%d validation issues:", len(v.Issues)))
	for _, i := range v.Issues {
		b.WriteString("\n  - ")
		b.WriteString(i.Field)
		b.WriteString(": ")
		b.WriteString(i.Message)
	}
	return b.String()
}

// Validate runs the load-time checks. Returns a *ValidationError if any
// issues were found.
func (c *Config) Validate() error {
	var issues []FieldIssue

	add := func(field, msg string) {
		issues = append(issues, FieldIssue{Field: field, Message: msg})
	}

	if c.App == "" {
		add("app", "required — short slug used in log lines and lock paths")
	} else if !isValidSlug(c.App) {
		add("app", fmt.Sprintf("%q must be lowercase alphanumerics with - or _ only", c.App))
	}

	if c.Host.SSH == "" {
		add("host.ssh", `required — e.g. "ubuntu@1.2.3.4" or "ubuntu@host:2222"`)
	} else if !isValidSSHTarget(c.Host.SSH) {
		add("host.ssh", fmt.Sprintf("%q must be in the form user@host[:port]", c.Host.SSH))
	}

	if c.Host.ReleaseRoot == "" {
		add("host.release_root", "required — absolute remote path (e.g. /var/www/app)")
	} else if !strings.HasPrefix(c.Host.ReleaseRoot, "/") {
		add("host.release_root", "must be an absolute path starting with /")
	}

	if c.Artifact.Source == "" {
		add("artifact.source", "required — local path to the build artifact")
	}
	switch c.Artifact.Format {
	case "zip", "tar.gz":
		// ok
	case "":
		add("artifact.format", fmt.Sprintf("could not infer format from source %q; specify zip or tar.gz", c.Artifact.Source))
	default:
		add("artifact.format", fmt.Sprintf("%q is not a supported format; use zip or tar.gz", c.Artifact.Format))
	}

	if c.Releases.Keep < 1 {
		add("releases.keep", "must be at least 1 (cannot prune the current release)")
	}

	if c.HealthCheck.URL != "" {
		if !strings.HasPrefix(c.HealthCheck.URL, "http://") && !strings.HasPrefix(c.HealthCheck.URL, "https://") {
			add("health_check.url", fmt.Sprintf("%q must start with http:// or https://", c.HealthCheck.URL))
		}
		if c.HealthCheck.Status < 100 || c.HealthCheck.Status > 599 {
			add("health_check.status", fmt.Sprintf("%d is not a valid HTTP status code", c.HealthCheck.Status))
		}
		if c.HealthCheck.Retries < 1 {
			add("health_check.retries", "must be at least 1")
		}
		if c.HealthCheck.Delay.Duration() <= 0 {
			add("health_check.delay", "must be positive (e.g. 3s)")
		}
		if c.HealthCheck.Timeout.Duration() <= 0 {
			add("health_check.timeout", "must be positive (e.g. 5s)")
		}
	}

	for i, hook := range c.Hooks.PreUpload {
		if msg, ok := checkShellSnippet(hook); !ok {
			add(fmt.Sprintf("hooks.pre_upload[%d]", i), msg)
		}
	}
	for i, hook := range c.Hooks.PostExtract {
		if msg, ok := checkShellSnippet(hook); !ok {
			add(fmt.Sprintf("hooks.post_extract[%d]", i), msg)
		}
	}
	for i, hook := range c.Hooks.PostFlip {
		if msg, ok := checkShellSnippet(hook); !ok {
			add(fmt.Sprintf("hooks.post_flip[%d]", i), msg)
		}
	}
	for i, hook := range c.Hooks.OnRollback {
		if msg, ok := checkShellSnippet(hook); !ok {
			add(fmt.Sprintf("hooks.on_rollback[%d]", i), msg)
		}
	}

	for i, f := range c.Shared.Files {
		if strings.HasPrefix(f, "/") {
			add(fmt.Sprintf("shared.files[%d]", i), fmt.Sprintf("%q must be a relative path (relative to the release dir)", f))
		}
	}
	for i, d := range c.Shared.Dirs {
		if strings.HasPrefix(d, "/") {
			add(fmt.Sprintf("shared.dirs[%d]", i), fmt.Sprintf("%q must be a relative path (relative to the release dir)", d))
		}
	}

	if c.Lock.IsEnabled() {
		if c.Lock.Path == "" {
			add("lock.path", "required when lock is enabled (defaults to <release_root>/shared/.shipyard.lock)")
		} else if !strings.HasPrefix(c.Lock.Path, "/") {
			add("lock.path", "must be an absolute path starting with /")
		}
		if c.Lock.TTL.Duration() < 30*time.Second {
			add("lock.ttl", "must be at least 30s to avoid stealing locks from in-flight deploys")
		}
	}

	if len(issues) > 0 {
		return &ValidationError{Issues: issues}
	}
	return nil
}

// isValidSlug accepts the same character set as Docker / Kubernetes names.
func isValidSlug(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
			if i == 0 || i == len(s)-1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// isValidSSHTarget does a sanity check on user@host[:port]. Not a full
// parser — the SSH client library will reject genuinely malformed values
// at connect time; this catches the typos that would confuse a user.
func isValidSSHTarget(s string) bool {
	at := strings.Index(s, "@")
	if at <= 0 || at == len(s)-1 {
		return false
	}
	user := s[:at]
	host := s[at+1:]
	if strings.ContainsAny(user, " \t\n") || strings.ContainsAny(host, " \t\n") {
		return false
	}
	return true
}

// checkShellSnippet does a lightweight balance check on single and double
// quotes. If quotes are unbalanced, the snippet would fail at sh -c time
// in a way that's confusing — better to flag at config-load time.
//
// Returns (errorMessage, false) when broken, ("", true) when OK.
func checkShellSnippet(snippet string) (string, bool) {
	if strings.TrimSpace(snippet) == "" {
		return "empty hook entry", false
	}
	if countUnescaped(snippet, '\'')%2 != 0 {
		return "unbalanced single quotes", false
	}
	if countUnescaped(snippet, '"')%2 != 0 {
		return "unbalanced double quotes", false
	}
	return "", true
}

func countUnescaped(s string, q rune) int {
	n := 0
	for i, r := range s {
		if r != q {
			continue
		}
		if i > 0 && s[i-1] == '\\' {
			continue
		}
		n++
	}
	return n
}
