// Package ssh wraps golang.org/x/crypto/ssh + pkg/sftp into a Client
// that's narrow enough for the deploy orchestrator to use. It is NOT a
// general-purpose SSH library — Run / RunStream / Upload / Exists / Stat
// / MkdirAll / RemoveAll are the only operations.
//
// A Client owns one underlying SSH connection and one SFTP subsystem.
// It is not safe for concurrent use — use one Client per deploy.
package ssh

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Client is the active SSH + SFTP session.
type Client struct {
	target  Target
	ssh     *ssh.Client
	sftp    *sftp.Client
	closeMu bool // poor man's "already closed" guard
}

// ConnectOpts configures a new Client.
type ConnectOpts struct {
	IdentityFile   string        // path to private key; ~ expanded; required
	KnownHostsFile string        // path to known_hosts; ~ expanded; default ~/.ssh/known_hosts
	Timeout        time.Duration // dial timeout; default 15s
	KeepAlive      time.Duration // SSH keep-alive interval; default 30s

	// AllowUnknownHosts skips host-key verification. Default false. Only
	// use in tests that target ephemeral containers.
	AllowUnknownHosts bool
}

// Connect dials the target host and opens an SFTP subsystem on top.
func Connect(ctx context.Context, target Target, opts ConnectOpts) (*Client, error) {
	if opts.IdentityFile == "" {
		return nil, errors.New("identity_file is required")
	}
	if opts.Timeout == 0 {
		opts.Timeout = 15 * time.Second
	}
	if opts.KeepAlive == 0 {
		opts.KeepAlive = 30 * time.Second
	}

	keyPath, err := expandPath(opts.IdentityFile)
	if err != nil {
		return nil, fmt.Errorf("expanding identity_file %q: %w", opts.IdentityFile, err)
	}
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("reading identity_file %s: %w", keyPath, err)
	}
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		// Try as an OpenSSH-format key with no passphrase. The bare
		// ParsePrivateKey error is unhelpful here; surface a hint.
		return nil, fmt.Errorf("parsing identity_file %s (encrypted keys not supported in v0.1 — decrypt with ssh-keygen -p first): %w", keyPath, err)
	}

	hostKeyCallback, err := buildHostKeyCallback(opts)
	if err != nil {
		return nil, fmt.Errorf("setting up host-key verification: %w", err)
	}

	cfg := &ssh.ClientConfig{
		User:            target.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKeyCallback,
		Timeout:         opts.Timeout,
		BannerCallback:  ssh.BannerDisplayStderr(),
	}

	var d net.Dialer
	d.Timeout = opts.Timeout
	conn, err := d.DialContext(ctx, "tcp", target.Addr())
	if err != nil {
		return nil, fmt.Errorf("dialing %s: %w", target.Addr(), err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, target.Addr(), cfg)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh handshake to %s: %w", target.Addr(), err)
	}

	sshClient := ssh.NewClient(sshConn, chans, reqs)

	if opts.KeepAlive > 0 {
		go keepAlive(ctx, sshClient, opts.KeepAlive)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("opening sftp subsystem on %s: %w", target.Addr(), err)
	}

	return &Client{target: target, ssh: sshClient, sftp: sftpClient}, nil
}

// Target returns the parsed destination.
func (c *Client) Target() Target { return c.target }

// SFTP returns the underlying SFTP client for callers that need it
// directly (rare; prefer the wrapper methods).
func (c *Client) SFTP() *sftp.Client { return c.sftp }

// Close shuts down the SFTP subsystem then the SSH connection.
func (c *Client) Close() error {
	if c.closeMu {
		return nil
	}
	c.closeMu = true

	var errs []error
	if c.sftp != nil {
		if err := c.sftp.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing sftp: %w", err))
		}
	}
	if c.ssh != nil {
		if err := c.ssh.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing ssh: %w", err))
		}
	}
	return errors.Join(errs...)
}

func buildHostKeyCallback(opts ConnectOpts) (ssh.HostKeyCallback, error) {
	if opts.AllowUnknownHosts {
		return ssh.InsecureIgnoreHostKey(), nil //nolint:gosec // documented test-only path
	}

	khPath := opts.KnownHostsFile
	if khPath == "" {
		home, err := userHomeDir()
		if err != nil {
			return nil, err
		}
		khPath = filepath.Join(home, ".ssh", "known_hosts")
	} else {
		expanded, err := expandPath(khPath)
		if err != nil {
			return nil, err
		}
		khPath = expanded
	}

	if _, err := os.Stat(khPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("known_hosts file %s does not exist — run `ssh-keyscan -H <host> >> %s` first", khPath, khPath)
		}
		return nil, fmt.Errorf("stat known_hosts: %w", err)
	}

	cb, err := knownhosts.New(khPath)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", khPath, err)
	}
	return cb, nil
}

func keepAlive(ctx context.Context, c *ssh.Client, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			// SendRequest is a no-op global ssh request — used as a
			// keepalive ping. Errors here mean the connection is gone.
			_, _, err := c.SendRequest("keepalive@shipyard", true, nil)
			if err != nil {
				return
			}
		}
	}
}

func expandPath(p string) (string, error) {
	if !strings.HasPrefix(p, "~") {
		return p, nil
	}
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	if p == "~" {
		return home, nil
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:]), nil
	}
	return "", fmt.Errorf("expanding %q: only ~/ is supported, not ~user/", p)
}

func userHomeDir() (string, error) {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h, nil
	}
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("resolving home dir: %w", err)
	}
	return u.HomeDir, nil
}
