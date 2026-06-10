package ssh

import (
	"bytes"
	"context"
	"os"
	"path"
	"strings"
	"testing"
	"time"
)

// TestIntegrationConnectExecUpload exercises a real SSH host. Skipped
// unless SHIPYARD_INTEGRATION_HOST and SHIPYARD_INTEGRATION_KEY are set.
//
// Usage:
//
//	SHIPYARD_INTEGRATION_HOST=ubuntu@1.2.3.4 \
//	SHIPYARD_INTEGRATION_KEY=~/.ssh/ps4_new \
//	go test -tags=integration -run TestIntegrationConnectExecUpload ./internal/ssh
//
// The test creates a temp file under /tmp on the remote, runs a command,
// verifies the output, and cleans up.
func TestIntegrationConnectExecUpload(t *testing.T) {
	host := os.Getenv("SHIPYARD_INTEGRATION_HOST")
	key := os.Getenv("SHIPYARD_INTEGRATION_KEY")
	if host == "" || key == "" {
		t.Skip("SHIPYARD_INTEGRATION_HOST + SHIPYARD_INTEGRATION_KEY not set; skipping")
	}

	target, err := ParseTarget(host)
	if err != nil {
		t.Fatalf("ParseTarget: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := Connect(ctx, target, ConnectOpts{
		IdentityFile: key,
		Timeout:      10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	// 1. Buffered Run.
	res, err := c.Run(ctx, "uname -s && whoami")
	if err != nil {
		t.Fatalf("Run uname/whoami: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("uname exit = %d; stderr = %q", res.ExitCode, res.Stderr)
	}
	if !strings.Contains(string(res.Stdout), "Linux") {
		t.Errorf("uname stdout missing Linux: %q", res.Stdout)
	}

	// 2. RunStream.
	var streamOut, streamErr bytes.Buffer
	exitCode, err := c.RunStream(ctx, "echo hello-shipyard", &streamOut, &streamErr)
	if err != nil {
		t.Fatalf("RunStream: %v", err)
	}
	if exitCode != 0 || !strings.Contains(streamOut.String(), "hello-shipyard") {
		t.Errorf("RunStream: exit=%d, stdout=%q", exitCode, streamOut.String())
	}

	// 3. Upload + Exists + RemoveAll.
	local := t.TempDir() + "/payload.txt"
	if err := os.WriteFile(local, []byte("shipyard integration test\n"), 0o600); err != nil {
		t.Fatalf("write local: %v", err)
	}
	remote := "/tmp/shipyard-integration-" + time.Now().Format("20060102150405")
	if err := c.Upload(ctx, local, path.Join(remote, "payload.txt")); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	ok, err := c.Exists(ctx, path.Join(remote, "payload.txt"))
	if err != nil || !ok {
		t.Fatalf("Exists after upload: ok=%v err=%v", ok, err)
	}
	if err := c.RemoveAll(ctx, remote); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	ok, err = c.Exists(ctx, remote)
	if err != nil || ok {
		t.Fatalf("Exists after RemoveAll: ok=%v err=%v", ok, err)
	}
}
