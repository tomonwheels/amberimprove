// Package improve runs frankl's improvefile procedure on one title:
// original into RAM, several passes on the target medium, bit-identity
// check after every pass, then an in-place replacement by rename.
//
// A pass is written by one of frankl's two programs (frankl_stereo,
// GPL-3.0-or-later), chosen in the configuration:
//   - bufhrt --interval: the original lies in RAM and is written from there
//     (the procedure of 0.1.x, still the default);
//   - improvedirect (since frankl_stereo 0.9.4, here since 13.11.2026): reads
//     and writes small blocks in turns, straight from and to the medium
//     (O_DIRECT), "instead of loading music data into RAM first" (CHANGES).
//
// The pass structure with verification after each pass follows Harald
// Scherer's nsc scripts (nihil.sine.causa, GPL-3.0-or-later) and is the same
// for both programs.
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

	// first is what pass 1 reads. bufhrt writes from the original in RAM.
	// improvedirect is made to read from the medium (direct read): its first
	// pass reads the title itself, which was just proven to be bit-identical
	// to the original; O_DIRECT on a RAM disk (tmpfs) would make no sense and
	// fails on kernels before 6.6. The title is only ever READ here — first
	// is never removed, the title is replaced by the final rename alone.
	first := orig
	if c.IsDirect() {
		first = target
	}
	prev := first
	delayed := 0
	cleanup := func() {
		if prev != first {
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
			n, err := RunPass(ctx, c, prev, out)
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
		if prev != first {
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
		Procedure:  c.Procedure,
		Passes:     c.Passes,
		Host:       host,
		Params:     Params(c),
		Delayed:    delayed,
	}, nil
}

// Params lists the parameters of the chosen procedure for the manifest.
func Params(c config.Config) map[string]any {
	if c.IsDirect() {
		d := c.Direct
		return map[string]any{
			"loops_per_second": d.LoopsPerSecond,
			"bytes_per_second": d.BytesPerSecond,
			"chunk_bytes":      d.ChunkSize(),
			"sync_loops":       d.SyncLoops,
			"out_copies":       d.OutCopies,
			"direct_write":     d.DirectWrite,
			"direct_read":      d.DirectRead,
			"dsync":            d.Dsync,
			"warmup":           d.Warmup,
			"preallocate":      d.Preallocate,
			"align":            d.EffectiveAlign(),
			"cpu":              d.CPU,
			"rt_prio":          d.RTPrio,
		}
	}
	b := c.Bufhrt
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

// DirectArgs builds the improvedirect command line, like the line in
// frankl's improvefile script (frankl_stereo v0.9.4, scripts/improvefile:21):
//
//	improvedirect -v -o -i -w "$1" "$2" 10 40960 10 128
//
// -v is added by RunDirect. The file names follow "--", so a title whose
// name starts with "-" is never read as an option.
func DirectArgs(d config.Direct, in, out string) []string {
	var a []string
	if d.CPU >= 0 {
		a = append(a, "taskset", "-c", strconv.Itoa(d.CPU))
	}
	if d.RTPrio > 0 {
		a = append(a, "chrt", "-f", strconv.Itoa(d.RTPrio))
	}
	a = append(a, d.Program)
	a = append(a, directSwitches(d)...)
	return append(a, "--", in, out,
		strconv.Itoa(d.LoopsPerSecond), strconv.FormatInt(d.BytesPerSecond, 10),
		strconv.Itoa(d.SyncLoops), strconv.Itoa(d.OutCopies))
}

// directSwitches are improvedirect's options in the order of its help text.
func directSwitches(d config.Direct) []string {
	var a []string
	if d.DirectWrite {
		a = append(a, "-o")
	}
	if d.Dsync {
		a = append(a, "-y")
	}
	if d.DirectRead {
		a = append(a, "-i")
	}
	if d.Align > 0 {
		a = append(a, "-a", strconv.Itoa(d.Align))
	}
	if d.Warmup {
		a = append(a, "-w")
	}
	if d.Preallocate {
		a = append(a, "-p")
	}
	return a
}

// RunDirect runs one improvedirect pass with -v and returns the number of
// timer resets it reports at the end ("timer reset N times"): how often the
// program fell behind its schedule and started the clock anew (a loop began
// late, or an fdatasync took longer than half a loop). It is improvedirect's
// counterpart to bufhrt's delayed blocks (0 = the timing was kept).
func RunDirect(ctx context.Context, d config.Direct, in, out string) (int, error) {
	a := DirectArgs(d, in, out)
	// -v goes in front of the "--" that ends the options.
	for i, s := range a {
		if s == "--" {
			a = append(a[:i], append([]string{"-v"}, a[i:]...)...)
			break
		}
	}
	cmd := exec.CommandContext(ctx, a[0], a[1:]...)
	cmd.WaitDelay = 5 * time.Second
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 0, err
	}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	resets := 0
	var tail []string
	sc := bufio.NewScanner(stderr)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.Index(line, "timer reset "); i >= 0 {
			_, _ = fmt.Sscanf(line[i:], "timer reset %d times", &resets)
		}
		tail = append(tail, line)
		if len(tail) > 5 {
			tail = tail[1:]
		}
	}
	if err := cmd.Wait(); err != nil {
		return resets, fmt.Errorf("improvedirect: %v: %s", err, strings.Join(tail, " | "))
	}
	return resets, nil
}

// RunPass writes one pass with the program of the chosen procedure and
// returns its count of timing misses.
func RunPass(ctx context.Context, c config.Config, in, out string) (int, error) {
	if c.IsDirect() {
		return RunDirect(ctx, c.Direct, in, out)
	}
	return RunBufhrt(ctx, c.Bufhrt, in, out)
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
