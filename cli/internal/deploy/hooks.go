package deploy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/philiprehberger/shipyard/cli/internal/logger"
	"github.com/philiprehberger/shipyard/cli/internal/ssh"
)

// runLocalHook runs cmd via `sh -c` on the local box, streaming output
// through the provided logger.
func runLocalHook(ctx context.Context, log *logger.Logger, cmd string) error {
	log.Info("hook", logAttr("cmd", cmd))

	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	out := &teeBuf{w: stdoutWriter{log: log}}
	errOut := &teeBuf{w: stderrWriter{log: log}}
	c.Stdout = out
	c.Stderr = errOut

	if err := c.Run(); err != nil {
		return fmt.Errorf("local hook %q failed: %w", cmd, err)
	}
	return nil
}

// runRemoteHook runs cmd via `sh -c` on the remote, streaming output
// through the logger. `cwd` is the remote working directory.
func runRemoteHook(ctx context.Context, log *logger.Logger, sshC *ssh.Client, cwd, cmd string) error {
	log.Info("hook", logAttr("cwd", cwd), logAttr("cmd", cmd))

	wrapped := fmt.Sprintf("cd %s && %s", shellQuote(cwd), cmd)

	out := stdoutWriter{log: log}
	errOut := stderrWriter{log: log}

	exitCode, err := sshC.RunStream(ctx, wrapped, out, errOut)
	if err != nil {
		return fmt.Errorf("remote hook transport error: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("remote hook %q exited %d", cmd, exitCode)
	}
	return nil
}

// shellQuote single-quotes s so it survives substitution into `sh -c`.
func shellQuote(s string) string {
	return "'" + replaceAll(s, "'", `'\''`) + "'"
}

func replaceAll(s, old, new string) string {
	if old == "" {
		return s
	}
	var b bytes.Buffer
	for {
		i := indexOf(s, old)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		b.WriteString(new)
		s = s[i+len(old):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// teeBuf forwards every Write to an inner writer and buffers the most
// recent N bytes so the deploy log can include the trailing output when
// a hook fails. We don't actually use the buffered tail today, but the
// shape is here for when we wire structured error reports.
type teeBuf struct {
	w   io.Writer
	buf bytes.Buffer
}

func (t *teeBuf) Write(p []byte) (int, error) {
	t.buf.Write(p)
	return t.w.Write(p)
}

// stdoutWriter / stderrWriter route hook output into the logger as Info /
// Warn records. Line-buffering would be nicer; for v0.1 each Write is
// emitted as-is.
type stdoutWriter struct{ log *logger.Logger }

func (s stdoutWriter) Write(p []byte) (int, error) {
	s.log.Info(trimTrailingNewline(string(p)))
	return len(p), nil
}

type stderrWriter struct{ log *logger.Logger }

func (s stderrWriter) Write(p []byte) (int, error) {
	s.log.Warn(trimTrailingNewline(string(p)))
	return len(p), nil
}

func trimTrailingNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
