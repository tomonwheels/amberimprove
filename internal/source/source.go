// Package source fetches the untouched original of a title into RAM.
package source

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tomonwheels/amberimprove/internal/config"
)

// Fetch copies the original of rel (path relative to the music root) to dst.
// Modification time is preserved (rsync -t / os.Chtimes). For kind "self" the
// file on the medium itself is the original.
func Fetch(ctx context.Context, c config.Config, rel, dst string) error {
	_ = os.Remove(dst)
	src := c.Source
	switch src.Kind {
	case "self":
		return copyLocal(filepath.Join(c.Music, rel), dst)
	case "rsync":
		ssh := shellJoin(append([]string{src.SSHProgram()}, src.SSHArgs()...))
		remote := strings.TrimSuffix(src.Remote, "/") + "/" + filepath.ToSlash(rel)
		// rsync >= 3.2.4 protects names with spaces, umlauts, & by default
		// (-s is refused by rrsync-restricted keys).
		cmd := exec.CommandContext(ctx, "rsync", "-t", "-e", ssh, remote, dst)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("rsync %s: %v: %s", rel, err, strings.TrimSpace(string(out)))
		}
		return nil
	case "path":
		return copyLocal(filepath.Join(src.Path, rel), dst)
	}
	return fmt.Errorf("unbekannte Quelle %q", src.Kind)
}

func copyLocal(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(to, st.ModTime(), st.ModTime())
}

// shellJoin quotes ssh arguments for rsync's -e string.
func shellJoin(args []string) string {
	q := make([]string, len(args))
	for i, a := range args {
		q[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return strings.Join(q, " ")
}
