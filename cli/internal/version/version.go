package version

// Version, Commit, and Date are set at build time via -ldflags:
//
//	go build -ldflags "-X github.com/philiprehberger/shipyard/cli/internal/version.Version=v0.1.0 \
//	                   -X github.com/philiprehberger/shipyard/cli/internal/version.Commit=$(git rev-parse --short HEAD) \
//	                   -X github.com/philiprehberger/shipyard/cli/internal/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
//	         ./cmd/shipyard
//
// GoReleaser sets these automatically per its config.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String renders the version triplet for `shipyard version` and for the
// rootCmd Version field (which Cobra surfaces on `--version`).
func String() string {
	return Version + " (" + Commit + ", " + Date + ")"
}
