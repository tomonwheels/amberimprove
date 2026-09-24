// Package improve runs frankl's improvefile procedure on one title:
// original into RAM, several bufhrt passes on the target medium, bit-identity
// check after every pass, then an in-place replacement by rename.
//
// The procedure is frankl's (bufhrt --interval, frankl_stereo, GPL-3.0-or-
// later); the pass structure with verification after each pass follows
// Harald Scherer's nsc scripts (nihil.sine.causa, GPL-3.0-or-later).
package improve

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tomonwheels/amberimprove/internal/cache"
	"github.com/tomonwheels/amberimprove/internal/config"
	"github.com/tomonwheels/amberimprove/internal/manifest"
	"github.com/tomonwheels/amberimprove/internal/source"
)

// Skip means the title was left untouched on purpose (reason key for the UI).
type Skip struct {
	Reason string
	Detail string
}

func (s *Skip) Error() string { return s.Reason + ": " + s.Detail }

// Skip reasons.
const (
	SkipSourceDiffers = "source_differs"
	SkipNoRAM         = "no_ram"
	SkipNoSpace       = "no_space"
	SkipFetch         = "fetch_failed"
)

// ErrNotIdentical: a pass stayed non-identical after all retries.
var ErrNotIdentical = errors.New("not_identical")

// Hooks report progress to the caller (state file, log).
type Hooks struct {
	Pass  func(pass int)
	Retry func(pass, try int)
	// Delayed reports blocks written late in a pass (timing misses).
	Delayed func(pass, n int)
}

// Title improves the title rel (relative to c.Music) and returns its
// manifest entry. On any error or cancellation the file on the SSD is left
// exactly as it was: it is only replaced by the final rename.
func Title(ctx context.Context, c config.Config, rel string, h Hooks) (manifest.Entry, error) {
	target := filepath.Join(c.Music, rel)
	tst, err := os.Stat(target)
	if err != nil {
		return manifest.Entry{}, err
	}
	size, mtime := tst.Size(), tst.ModTime()

	if err := os.MkdirAll(c.RAMDir, 0o700); err != nil {
		return manifest.Entry{}, err
	}
	if err := os.MkdirAll(c.WorkDir, 0o700); err != nil {
		return manifest.Entry{}, err
	}
	if free(c.RAMDir) < uint64(size)+64<<20 {
		return manifest.Entry{}, &Skip{SkipNoRAM, fmt.Sprintf("%d Bytes", size)}
	}
	if free(c.WorkDir) < 2*uint64(size)+256<<20 {
		return manifest.Entry{}, &Skip{SkipNoSpace, fmt.Sprintf("%d Bytes", size)}
	}
	clean(c.WorkDir)

	orig := filepath.Join(c.RAMDir, "original")
	defer os.Remove(orig)
	if err := source.Fetch(ctx, c, rel, orig); err != nil {
		if ctx.Err() != nil {
			return manifest.Entry{}, ctx.Err()
		}
		return manifest.Entry{}, &Skip{SkipFetch, err.Error()}
	}

	// Cross-check: the original must be exactly what is on the SSD.
	ost, err := os.Stat(orig)
	if err != nil {
		return manifest.Entry{}, err
	}
	if ost.Size() != size {
		return manifest.Entry{}, &Skip{SkipSourceDiffers, fmt.Sprintf("Größe %d ≠ %d", ost.Size(), size)}
	}
	if d := ost.ModTime().Sub(mtime); d > time.Duration(c.MtimeTolerance)*time.Second || -d > time.Duration(c.MtimeTolerance)*time.Second {
		return manifest.Entry{}, &Skip{SkipSourceDiffers, fmt.Sprintf("Zeit %s ≠ %s", ost.ModTime().Format(time.RFC3339), mtime.Format(time.RFC3339))}
	}
	sumOrig, err := sum(orig)
	if err != nil {
		return manifest.Entry{}, err
	}
	_ = cache.Evict(target)
	sumTarget, err := sum(target)
	if err != nil {
		return manifest.Entry{}, err
	}
	if sumOrig != sumTarget {
		return manifest.Entry{}, &Skip{SkipSourceDiffers, "SHA-256 weicht ab"}
	}

	prev := orig
	delayed := 0
	cleanup := func() {
		if prev != orig {
			os.Remove(prev)
		}
		clean(c.WorkDir)
	}
	for pass := 1; pass <= c.Passes; pass++ {
		if h.Pass != nil {
			h.Pass(pass)
		}
		out := filepath.Join(c.WorkDir, "durchgang"+strconv.Itoa(pass))
		ok := false
		for try := 1; try <= c.Retries; try++ {
			if try > 1 && h.Retry != nil {
				h.Retry(pass, try)
			}
			os.Remove(out)
			n, err := RunBufhrt(ctx, c.Bufhrt, prev, out)
			delayed += n
			if h.Delayed != nil && n > 0 {
				h.Delayed(pass, n)
			}
			if err != nil {
				os.Remove(out)
				cleanup()
				if ctx.Err() != nil {
					return manifest.Entry{}, ctx.Err()
				}
				return manifest.Entry{}, err
			}
			if err := cache.Evict(out); err != nil {
				os.Remove(out)
				cleanup()
				return manifest.Entry{}, err
			}
			got, err := sum(out)
			if err == nil && got == sumOrig {
				ok = true
				break
			}
		}
		if !ok {
			os.Remove(out)
			cleanup()
			return manifest.Entry{}, fmt.Errorf("%w: Durchgang %d", ErrNotIdentical, pass)
		}
		if prev != orig {
			os.Remove(prev)
		}
		prev = out
	}

	// Give the improved file the original's mode, owner and time, then
	// replace the original. rename only rewrites metadata, not the data.
	if err := os.Chmod(prev, tst.Mode().Perm()); err != nil {
		cleanup()
		return manifest.Entry{}, err
	}
	if s, ok := tst.Sys().(*syscall.Stat_t); ok {
		if err := os.Chown(prev, int(s.Uid), int(s.Gid)); err != nil {
			cleanup()
			return manifest.Entry{}, err
		}
	}
	if err := os.Chtimes(prev, mtime, mtime); err != nil {
		cleanup()
		return manifest.Entry{}, err
	}
	if err := os.Rename(prev, target); err != nil {
		cleanup()
		return manifest.Entry{}, err
	}
	_ = cache.Evict(target)

	host, _ := os.Hostname()
	return manifest.Entry{
		Size:       size,
		Mtime:      mtime,
		SHA256:     sumOrig,
		ImprovedAt: time.Now(),
		Method:     c.Method,
		Passes:     c.Passes,
		Host:       host,
		Params:     Params(c.Bufhrt),
		Delayed:    delayed,
	}, nil
}

// Params lists the bufhrt parameters for the manifest.
func Params(b config.Bufhrt) map[string]any {
	return map[string]any{
		"buffer_size":          b.BufferSize,
		"loops_per_second":     b.LoopsPerSecond,
		"bytes_per_second":     b.BytesPerSecond,
		"number_copies":        b.NumberCopies,
		"ram_loops_per_second": b.RAMLoopsPerSecond,
		"ram_bytes_per_second": b.RAMBytesPerSecond,
		"dsyncs_per_second":    b.DsyncsPerSecond,
		"out_copies":           b.OutCopies,
		"shift":                b.Shift,
		"cpu":                  b.CPU,
		"rt_prio":              b.RTPrio,
	}
}

// Args builds the command line, like frankl's improvefile script.
func Args(b config.Bufhrt, in, out string) []string {
	var a []string
	if b.CPU >= 0 {
		a = append(a, "taskset", "-c", strconv.Itoa(b.CPU))
	}
	if b.RTPrio > 0 {
		a = append(a, "chrt", "-f", strconv.Itoa(b.RTPrio))
	}
	a = append(a, b.Program, "--interval",
		"--file="+in, "--outfile="+out,
		fmt.Sprintf("--buffer-size=%d", b.BufferSize),
		fmt.Sprintf("--loops-per-second=%d", b.LoopsPerSecond),
		fmt.Sprintf("--bytes-per-second=%d", b.BytesPerSecond),
		fmt.Sprintf("--number-copies=%d", b.NumberCopies),
		fmt.Sprintf("--dsyncs-per-second=%d", b.DsyncsPerSecond),
		fmt.Sprintf("--out-copies=%d", b.OutCopies))
	if b.RAMLoopsPerSecond > 0 && b.RAMBytesPerSecond > 0 {
		a = append(a,
			fmt.Sprintf("--ram-loops-per-second=%d", b.RAMLoopsPerSecond),
			fmt.Sprintf("--ram-bytes-per-second=%d", b.RAMBytesPerSecond))
	}
	if b.Shift > 0 {
		a = append(a, fmt.Sprintf("--shift=%d", b.Shift))
	}
	return a
}

// RunBufhrt runs one pass with --verbose and returns the number of
// "delayed block" messages: blocks bufhrt could not write on time. That is
// the measure of how evenly the pass was written (0 = every block on time).
func RunBufhrt(ctx context.Context, b config.Bufhrt, in, out string) (int, error) {
	a := append(Args(b, in, out), "--verbose")
	cmd := exec.CommandContext(ctx, a[0], a[1:]...)
	cmd.WaitDelay = 5 * time.Second
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 0, err
	}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	delayed := 0
	var tail []string
	sc := bufio.NewScanner(stderr)
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, "delayed block") {
			delayed++
			continue
		}
		tail = append(tail, line)
		if len(tail) > 5 {
			tail = tail[1:]
		}
	}
	if err := cmd.Wait(); err != nil {
		return delayed, fmt.Errorf("bufhrt: %v: %s", err, strings.Join(tail, " | "))
	}
	return delayed, nil
}

func sum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// clean removes leftovers of an interrupted run from the work directory.
func clean(dir string) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "durchgang") {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

func free(dir string) uint64 {
	var s syscall.Statfs_t
	if syscall.Statfs(dir, &s) != nil {
		return 0
	}
	return uint64(s.Bavail) * uint64(s.Bsize)
}
