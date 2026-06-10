package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/ssh"
)

// RunResult is the captured output of a buffered remote command.
type RunResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

// Run executes cmd on the remote and buffers its stdout/stderr until exit.
// Use RunStream for long-running commands whose output should be visible
// as it arrives.
//
// A non-zero exit code is NOT returned as an error — the caller decides
// whether that's failure (e.g. health-check) or expected (e.g. test if
// a file exists).
func (c *Client) Run(ctx context.Context, cmd string) (*RunResult, error) {
	sess, err := c.ssh.NewSession()
	if err != nil {
		return nil, fmt.Errorf("opening ssh session: %w", err)
	}
	defer sess.Close()

	var stdout, stderr cappedBuffer
	stdout.cap = 4 * 1024 * 1024 // 4 MB
	stderr.cap = 4 * 1024 * 1024
	sess.Stdout = &stdout
	sess.Stderr = &stderr

	if err := runWithContext(ctx, sess, cmd); err != nil {
		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) {
			return &RunResult{
				ExitCode: exitErr.ExitStatus(),
				Stdout:   stdout.Bytes(),
				Stderr:   stderr.Bytes(),
			}, nil
		}
		return nil, fmt.Errorf("remote command %q: %w", cmd, err)
	}

	return &RunResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}, nil
}

// RunStream executes cmd and copies stdout/stderr to the provided writers
// as they arrive. The integer return is the remote exit code; the error
// is non-nil only for transport-level failures (the caller distinguishes
// non-zero exit from broken transport).
func (c *Client) RunStream(ctx context.Context, cmd string, stdout, stderr io.Writer) (int, error) {
	sess, err := c.ssh.NewSession()
	if err != nil {
		return -1, fmt.Errorf("opening ssh session: %w", err)
	}
	defer sess.Close()

	if stdout != nil {
		sess.Stdout = stdout
	}
	if stderr != nil {
		sess.Stderr = stderr
	}

	if err := runWithContext(ctx, sess, cmd); err != nil {
		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitStatus(), nil
		}
		return -1, fmt.Errorf("remote command %q: %w", cmd, err)
	}
	return 0, nil
}

// runWithContext runs sess.Run(cmd) but signals the session on ctx.Done.
// ssh.Session doesn't accept a Context directly — we wrap it in a goroutine
// that listens for cancellation and sends SIGTERM to the remote process.
func runWithContext(ctx context.Context, sess *ssh.Session, cmd string) error {
	done := make(chan error, 1)
	go func() {
		done <- sess.Run(cmd)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = sess.Signal(ssh.SIGTERM)
		select {
		case err := <-done:
			if err == nil {
				return ctx.Err()
			}
			return fmt.Errorf("%w (canceled mid-execution): %w", ctx.Err(), err)
		case <-time.After(5 * time.Second):
			_ = sess.Signal(ssh.SIGKILL)
			return fmt.Errorf("%w (force-killed after grace period)", ctx.Err())
		}
	}
}

// cappedBuffer is a write-once buffer that drops further data past cap.
// Used for buffered Run() captures so a runaway command (`yes`) doesn't
// OOM the deploy.
type cappedBuffer struct {
	buf []byte
	cap int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	remaining := b.cap - len(b.buf)
	if remaining <= 0 {
		return len(p), nil // pretend we wrote it
	}
	if len(p) > remaining {
		b.buf = append(b.buf, p[:remaining]...)
		return len(p), nil
	}
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *cappedBuffer) Bytes() []byte { return b.buf }
