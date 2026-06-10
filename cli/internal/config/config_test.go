package config

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const minimalValidYAML = `
app: webhook-relay
host:
  ssh: ubuntu@1.2.3.4
  release_root: /var/www/webhook-relay
artifact:
  source: ./build/release.zip
`

func TestLoadMinimalValid(t *testing.T) {
	t.Parallel()

	cfg, err := loadFromString(t, minimalValidYAML)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.App != "webhook-relay" {
		t.Errorf("App = %q; want webhook-relay", cfg.App)
	}
	if cfg.Artifact.Format != "zip" {
		t.Errorf("inferred format = %q; want zip", cfg.Artifact.Format)
	}
	if cfg.Releases.Keep != 5 {
		t.Errorf("Releases.Keep default = %d; want 5", cfg.Releases.Keep)
	}
	if !cfg.Lock.IsEnabled() {
		t.Error("Lock should default to enabled")
	}
	if cfg.Lock.Path != "/var/www/webhook-relay/shared/.shipyard.lock" {
		t.Errorf("Lock.Path = %q; want /var/www/webhook-relay/shared/.shipyard.lock", cfg.Lock.Path)
	}
	if cfg.Lock.TTL.Duration() != 10*time.Minute {
		t.Errorf("Lock.TTL = %v; want 10m", cfg.Lock.TTL.Duration())
	}
}

func TestLoadAppliesHealthCheckDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := loadFromString(t, minimalValidYAML+`
health_check:
  url: https://example.com/healthz
`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.HealthCheck.Status != 200 {
		t.Errorf("Status default = %d; want 200", cfg.HealthCheck.Status)
	}
	if cfg.HealthCheck.Retries != 10 {
		t.Errorf("Retries default = %d; want 10", cfg.HealthCheck.Retries)
	}
	if cfg.HealthCheck.Delay.Duration() != 3*time.Second {
		t.Errorf("Delay default = %v; want 3s", cfg.HealthCheck.Delay.Duration())
	}
	if cfg.HealthCheck.Timeout.Duration() != 5*time.Second {
		t.Errorf("Timeout default = %v; want 5s", cfg.HealthCheck.Timeout.Duration())
	}
}

func TestLoadAllowsExplicitLockDisable(t *testing.T) {
	t.Parallel()

	cfg, err := loadFromString(t, minimalValidYAML+`
lock:
  enabled: false
`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Lock.IsEnabled() {
		t.Error("explicit enabled: false must disable lock")
	}
}

func TestLoadParsesDurationStrings(t *testing.T) {
	t.Parallel()

	cfg, err := loadFromString(t, minimalValidYAML+`
health_check:
  url: https://example.com/healthz
  delay: 7s
  timeout: 2s
`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HealthCheck.Delay.Duration() != 7*time.Second {
		t.Errorf("delay = %v; want 7s", cfg.HealthCheck.Delay.Duration())
	}
	if cfg.HealthCheck.Timeout.Duration() != 2*time.Second {
		t.Errorf("timeout = %v; want 2s", cfg.HealthCheck.Timeout.Duration())
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	// "health-check" with a hyphen instead of an underscore is the bug we
	// want to catch — without strict-mode it'd silently fall through and
	// leave HealthCheck zero-valued.
	_, err := loadFromString(t, minimalValidYAML+`
health-check:
  url: https://example.com/healthz
`)
	if err == nil {
		t.Fatal("expected strict-mode rejection of unknown field 'health-check'")
	}
	if !strings.Contains(err.Error(), "health-check") {
		t.Errorf("error must mention the unknown field; got %v", err)
	}
}

func TestLoadRejectsEmptyFile(t *testing.T) {
	t.Parallel()

	_, err := loadFromString(t, "")
	if err == nil {
		t.Fatal("expected empty-file rejection")
	}
}

func TestLoadInfersTarGzFormat(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"./build/release.tar.gz": "tar.gz",
		"./build/release.tgz":    "tar.gz",
		"./build/release.zip":    "zip",
		"./build/release.ZIP":    "zip",
	}
	for source, want := range cases {
		cfg, err := loadFromString(t, `
app: app
host:
  ssh: ubuntu@1.2.3.4
  release_root: /var/www/app
artifact:
  source: `+source+`
`)
		if err != nil {
			t.Errorf("source %s: load: %v", source, err)
			continue
		}
		if cfg.Artifact.Format != want {
			t.Errorf("source %s: format = %q; want %q", source, cfg.Artifact.Format, want)
		}
	}
}

func TestValidateRequiresAllRequiredFields(t *testing.T) {
	t.Parallel()

	// Each row: a partial YAML doc; the field name expected to appear in
	// the validation error.
	cases := []struct {
		name      string
		yaml      string
		wantField string
	}{
		{
			name: "missing app",
			yaml: `
host:
  ssh: ubuntu@1.2.3.4
  release_root: /var/www/app
artifact:
  source: ./build/release.zip
`,
			wantField: "app",
		},
		{
			name: "missing host.ssh",
			yaml: `
app: app
host:
  release_root: /var/www/app
artifact:
  source: ./build/release.zip
`,
			wantField: "host.ssh",
		},
		{
			name: "missing host.release_root",
			yaml: `
app: app
host:
  ssh: ubuntu@1.2.3.4
artifact:
  source: ./build/release.zip
`,
			wantField: "host.release_root",
		},
		{
			name: "relative release_root",
			yaml: `
app: app
host:
  ssh: ubuntu@1.2.3.4
  release_root: var/www/app
artifact:
  source: ./build/release.zip
`,
			wantField: "host.release_root",
		},
		{
			name: "missing artifact.source",
			yaml: `
app: app
host:
  ssh: ubuntu@1.2.3.4
  release_root: /var/www/app
`,
			wantField: "artifact.source",
		},
		{
			name: "bad ssh target",
			yaml: `
app: app
host:
  ssh: not-an-ssh-target
  release_root: /var/www/app
artifact:
  source: ./build/release.zip
`,
			wantField: "host.ssh",
		},
		{
			name: "bad slug",
			yaml: `
app: "Webhook Relay!"
host:
  ssh: ubuntu@1.2.3.4
  release_root: /var/www/app
artifact:
  source: ./build/release.zip
`,
			wantField: "app",
		},
		{
			name: "bad artifact format",
			yaml: `
app: app
host:
  ssh: ubuntu@1.2.3.4
  release_root: /var/www/app
artifact:
  source: ./build/release.rar
  format: rar
`,
			wantField: "artifact.format",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := loadFromString(t, tc.yaml)
			if err == nil {
				t.Fatalf("expected validation failure for %s", tc.name)
			}
			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("expected *ValidationError, got %T: %v", err, err)
			}
			found := false
			for _, issue := range verr.Issues {
				if strings.HasPrefix(issue.Field, tc.wantField) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("no issue mentioned field %q; got %v", tc.wantField, verr.Issues)
			}
		})
	}
}

func TestValidateCatchesUnbalancedQuotes(t *testing.T) {
	t.Parallel()

	_, err := loadFromString(t, minimalValidYAML+`
hooks:
  pre_upload:
    - "echo 'unterminated"
`)
	if err == nil {
		t.Fatal("expected unbalanced-quote rejection")
	}
	if !strings.Contains(err.Error(), "pre_upload") {
		t.Errorf("error must point at the broken hook; got %v", err)
	}
}

func TestValidateLockTTLMustBeReasonable(t *testing.T) {
	t.Parallel()

	_, err := loadFromString(t, minimalValidYAML+`
lock:
  enabled: true
  path: /var/www/webhook-relay/shared/.shipyard.lock
  ttl: 5s
`)
	if err == nil {
		t.Fatal("expected TTL-too-short rejection")
	}
	if !strings.Contains(err.Error(), "lock.ttl") {
		t.Errorf("error must point at lock.ttl; got %v", err)
	}
}

func TestExampleFilesParse(t *testing.T) {
	t.Parallel()

	matches, err := filepath.Glob("../../../examples/*.yaml")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) == 0 {
		t.Skip("no example configs to test yet")
	}
	for _, path := range matches {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			if cfg.App == "" {
				t.Errorf("%s: App is empty after Load", path)
			}
		})
	}
}

// loadFromString is the test helper most other tests use. It writes the
// YAML to a temp file, calls Load, and removes the file when the test
// completes. Path resolution code in Load behaves the same as in
// production this way.
func loadFromString(t *testing.T, contents string) (*Config, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "shipyard.yaml")
	if err := writeFile(path, contents); err != nil {
		t.Fatalf("write tmp file: %v", err)
	}
	return Load(path)
}
