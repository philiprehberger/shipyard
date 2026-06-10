package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
)

// Upload streams a local file to the remote host via SFTP. The remote
// directory is created if it doesn't exist. The function honors context
// cancellation between chunks.
func (c *Client) Upload(ctx context.Context, localPath, remotePath string) error {
	if err := c.MkdirAll(ctx, path.Dir(remotePath)); err != nil {
		return err
	}

	src, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("opening local file %s: %w", localPath, err)
	}
	defer src.Close()

	dst, err := c.sftp.Create(remotePath)
	if err != nil {
		return fmt.Errorf("creating remote file %s: %w", remotePath, err)
	}
	defer dst.Close()

	if _, err := copyWithContext(ctx, dst, src); err != nil {
		// Best-effort cleanup so a partial transfer doesn't sit on the
		// remote forever pretending to be a valid release.
		_ = c.sftp.Remove(remotePath)
		return fmt.Errorf("uploading %s: %w", localPath, err)
	}
	return nil
}

// Exists returns true if path exists on the remote. Symlinks are followed.
func (c *Client) Exists(ctx context.Context, remotePath string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	_, err := c.sftp.Stat(remotePath)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("stat %s: %w", remotePath, err)
}

// MkdirAll creates remotePath and all missing parents. SFTP's stdlib
// has Mkdir but not MkdirAll — we walk the path and create each segment.
func (c *Client) MkdirAll(ctx context.Context, remotePath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if remotePath == "" || remotePath == "/" || remotePath == "." {
		return nil
	}

	// Use MkdirAll if pkg/sftp gave us one (newer versions do).
	if err := c.sftp.MkdirAll(remotePath); err != nil {
		return fmt.Errorf("mkdir -p %s: %w", remotePath, err)
	}
	return nil
}

// RemoveAll deletes remotePath recursively. Safe for both files and dirs.
// Used by the rollback / prune flows. Returns nil if path doesn't exist.
func (c *Client) RemoveAll(ctx context.Context, remotePath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	info, err := c.sftp.Stat(remotePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", remotePath, err)
	}

	if !info.IsDir() {
		return c.sftp.Remove(remotePath)
	}

	entries, err := c.sftp.ReadDir(remotePath)
	if err != nil {
		return fmt.Errorf("listing %s: %w", remotePath, err)
	}
	for _, e := range entries {
		if err := c.RemoveAll(ctx, path.Join(remotePath, e.Name())); err != nil {
			return err
		}
	}
	return c.sftp.RemoveDirectory(remotePath)
}

// copyWithContext is io.Copy with periodic ctx checks. Chunks at 1 MiB.
func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	const chunk = 1 << 20
	buf := make([]byte, chunk)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, err := src.Read(buf)
		if n > 0 {
			written, werr := dst.Write(buf[:n])
			total += int64(written)
			if werr != nil {
				return total, werr
			}
		}
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}
