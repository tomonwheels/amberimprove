package improve

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tomonwheels/amberimprove/internal/config"
)

// The stand-ins take the place of frankl's programs (Linux programs with
// precise timing — not what these tests are about). They copy like the real
// ones, write every call into $ATTRAPPE_LOG and behave as $ATTRAPPE_ART says:
//
//	gut      copy, report "timer reset 2 times" like improvedirect -v
//	falsch   copy with one byte too many (not bit-identical)
//	einmal   not bit-identical on the first call, then good
//	haengt   write a part, then hang (until killed)
//	zweiter  good on the first call, hang on the second
//	fehler   exit with an error like improvedirect
const directAttrappe = `#!/bin/sh
printf '%s\n' "$*" >> "$ATTRAPPE_LOG"
while [ "$1" != "--" ]; do shift; done
in="$2"; out="$3"
case "$ATTRAPPE_ART" in
gut)    cat "$in" > "$out"
        echo "improvedirect: 4096 bytes per loop (40960 bytes/s), output O_DIRECT, input O_DIRECT, warm-up 1, preallocate 0" >&2
        echo "improvedirect: 5 bytes written, 1 fdatasync, timer reset 2 times" >&2 ;;
falsch) cat "$in" > "$out"; printf X >> "$out" ;;
einmal) cat "$in" > "$out"
        if [ ! -e "$ATTRAPPE_LOG.einmal" ]; then : > "$ATTRAPPE_LOG.einmal"; printf X >> "$out"; fi ;;
haengt) head -c 10 "$in" > "$out"; exec sleep 60 ;;
zweiter) if [ ! -e "$ATTRAPPE_LOG.zweiter" ]; then : > "$ATTRAPPE_LOG.zweiter"; cat "$in" > "$out"
        else head -c 10 "$in" > "$out"; exec sleep 60; fi ;;
fehler) echo "improvedirect: Cannot open output file $out." >&2; exit 3 ;;
esac
`

// bufhrtAttrappe copies --file to --outfile like bufhrt --interval.
const bufhrtAttrappe = `#!/bin/sh
printf '%s\n' "$*" >> "$ATTRAPPE_LOG"
for a in "$@"; do
  case "$a" in
  --file=*) in="${a#--file=}" ;;
  --outfile=*) out="${a#--outfile=}" ;;
  esac
done
cat "$in" > "$out"
`

type bench struct {
	cfg    config.Config
	rel    string
	target string
	data   []byte
	before os.FileInfo
	log    string
}

// setup builds a music folder with one title and a configuration whose
// programs are the stand-ins.
func setup(t *testing.T, procedure, art string) bench {
	t.Helper()
	dir := t.TempDir()
	b := bench{rel: "Album/01 Titel.flac", log: filepath.Join(dir, "aufrufe.log")}
	c := config.Default()
	c.Music = filepath.Join(dir, "musik")
	c.WorkDir = filepath.Join(dir, ".amberimprove-tmp")
	c.RAMDir = filepath.Join(dir, "ram")
	c.StateDir = filepath.Join(dir, "zustand")
	c.Passes, c.Retries = 2, 3
	c.Procedure = procedure
	if err := c.ApplyPreset("sehr-langsam"); err != nil {
		t.Fatal(err)
	}
	c.Direct.Program = filepath.Join(dir, "improvedirect")
	c.Bufhrt.Program = filepath.Join(dir, "bufhrt")
	if err := os.WriteFile(c.Direct.Program, []byte(directAttrappe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.Bufhrt.Program, []byte(bufhrtAttrappe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	b.cfg = c
	b.target = filepath.Join(c.Music, b.rel)
	b.data = bytes.Repeat([]byte("amberIMPROVE Probe\n"), 5000)
	if err := os.MkdirAll(filepath.Dir(b.target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b.target, b.data, 0o640); err != nil {
		t.Fatal(err)
	}
	mt := time.Date(2024, 5, 2, 10, 11, 12, 0, time.UTC)
	if err := os.Chtimes(b.target, mt, mt); err != nil {
		t.Fatal(err)
	}
	var err error
	if b.before, err = os.Stat(b.target); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ATTRAPPE_LOG", b.log)
	t.Setenv("ATTRAPPE_ART", art)
	return b
}

// calls returns the logged command lines of the stand-in.
func (b bench) calls(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(b.log)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

// untouched: the title is still the very same file (not replaced, not
// rewritten) and nothing is left in the work and RAM directories.
func (b bench) untouched(t *testing.T) {
	t.Helper()
	after, err := os.Stat(b.target)
	if err != nil {
		t.Fatal("the title is gone:", err)
	}
	if !os.SameFile(b.before, after) || !after.ModTime().Equal(b.before.ModTime()) || after.Size() != b.before.Size() {
		t.Fatal("the title was replaced or changed")
	}
	if got, _ := os.ReadFile(b.target); !bytes.Equal(got, b.data) {
		t.Fatal("the title's content changed")
	}
	b.clean(t)
}

// clean: no pass files and no original left behind.
func (b bench) clean(t *testing.T) {
	t.Helper()
	for _, dir := range []string{b.cfg.WorkDir, b.cfg.RAMDir} {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			t.Errorf("left behind: %s", filepath.Join(dir, e.Name()))
		}
	}
}

// frankl's line (scripts/improvefile, v0.9.4):
// improvedirect -v -o -i -w "$1" "$2" 10 40960 10 128
func TestDirectArgs(t *testing.T) {
	d := config.DirectPresets[0].Direct
	d.Program, d.CPU = "/opt/amberimprove/bin/improvedirect", -1
	got := DirectArgs(d, "/m/a.flac", "/w/durchgang1")
	want := []string{"/opt/amberimprove/bin/improvedirect", "-o", "-i", "-w", "--", "/m/a.flac", "/w/durchgang1", "10", "40960", "10", "128"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frankl's set:\n got %q\nwant %q", got, want)
	}
	// Every switch, pinning and priority.
	d = config.Direct{Program: "/p/improvedirect", CPU: 2, RTPrio: 60, LoopsPerSecond: 20, BytesPerSecond: 81920,
		SyncLoops: 5, OutCopies: 2, DirectWrite: true, DirectRead: true, Dsync: true, Warmup: true, Preallocate: true, Align: 512}
	got = DirectArgs(d, "in", "out")
	want = []string{"taskset", "-c", "2", "chrt", "-f", "60", "/p/improvedirect", "-o", "-y", "-i", "-a", "512", "-w", "-p", "--", "in", "out", "20", "81920", "5", "2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("all switches:\n got %q\nwant %q", got, want)
	}
	// No switch at all: plain reading and writing through the page cache.
	got = DirectArgs(config.Direct{Program: "/p/improvedirect", CPU: -1, LoopsPerSecond: 1, BytesPerSecond: 2, SyncLoops: 3, OutCopies: 4}, "in", "out")
	want = []string{"/p/improvedirect", "--", "in", "out", "1", "2", "3", "4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("no switches:\n got %q\nwant %q", got, want)
	}
}

// The bufhrt command line is the one of 0.1.x.
func TestBufhrtArgsUnchanged(t *testing.T) {
	c := config.Default()
	got := strings.Join(Args(c.Bufhrt, "in", "out"), " ")
	want := "/opt/amberimprove/bin/bufhrt --interval --file=in --outfile=out --buffer-size=419430400 " +
		"--loops-per-second=10 --bytes-per-second=188044 --number-copies=1 --dsyncs-per-second=1 --out-copies=256 " +
		"--ram-loops-per-second=100 --ram-bytes-per-second=1880440"
	if got != want {
		t.Fatalf("bufhrt:\n got %s\nwant %s", got, want)
	}
}

func TestParams(t *testing.T) {
	c := config.Default()
	if p := Params(c); p["buffer_size"] != int64(419430400) || p["out_copies"] != 256 || len(p) != 11 {
		t.Fatalf("bufhrt params: %v", p)
	}
	c.Procedure = config.ProcDirect
	if err := c.ApplyPreset("sehr-langsam"); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"loops_per_second": 10, "bytes_per_second": int64(40960), "chunk_bytes": int64(4096),
		"sync_loops": 10, "out_copies": 128, "direct_write": true, "direct_read": true,
		"dsync": false, "warmup": true, "preallocate": false, "align": int64(4096), "cpu": -1, "rt_prio": 0,
	}
	if p := Params(c); !reflect.DeepEqual(p, want) {
		t.Fatalf("direct params:\n got %v\nwant %v", p, want)
	}
}

// The pass structure with improvedirect: pass 1 reads the title from the
// medium, every further pass the file of the pass before; bit-identity after
// every pass; the title is replaced by rename at the very end.
func TestTitleDirect(t *testing.T) {
	b := setup(t, config.ProcDirect, "gut")
	var passes []int
	var delayed []int
	e, err := Title(context.Background(), b.cfg, b.rel, Hooks{
		Pass:    func(p int) { passes = append(passes, p) },
		Delayed: func(p, n int) { delayed = append(delayed, n) },
	})
	if err != nil {
		t.Fatal(err)
	}
	p1, p2 := filepath.Join(b.cfg.WorkDir, "durchgang1"), filepath.Join(b.cfg.WorkDir, "durchgang2")
	want := []string{
		"-o -i -w -v -- " + b.target + " " + p1 + " 10 40960 10 128",
		"-o -i -w -v -- " + p1 + " " + p2 + " 10 40960 10 128",
	}
	if got := b.calls(t); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls:\n got %q\nwant %q", got, want)
	}
	if !reflect.DeepEqual(passes, []int{1, 2}) || !reflect.DeepEqual(delayed, []int{2, 2}) {
		t.Fatalf("hooks: passes %v, timer resets %v", passes, delayed)
	}
	// Replaced by the last pass file: another file, same content, mode, time.
	after, err := os.Stat(b.target)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(b.before, after) {
		t.Fatal("the title was not replaced")
	}
	if got, _ := os.ReadFile(b.target); !bytes.Equal(got, b.data) {
		t.Fatal("the improved title is not bit-identical")
	}
	if after.Mode().Perm() != 0o640 || !after.ModTime().Equal(b.before.ModTime()) {
		t.Fatalf("mode %v, time %v", after.Mode().Perm(), after.ModTime())
	}
	b.clean(t)
	if e.Method != "frankl-0.9.4/direkt/sehr-langsam/2x" || e.Procedure != config.ProcDirect || e.Passes != 2 || e.Delayed != 4 {
		t.Fatalf("entry: %+v", e)
	}
	if e.Size != int64(len(b.data)) || len(e.SHA256) != 64 || e.Params["out_copies"] != 128 || e.Params["direct_read"] != true {
		t.Fatalf("entry: %+v", e)
	}
}

// A pass that is not bit-identical is repeated; the title only counts when
// the repetition is.
func TestTitleDirectRetry(t *testing.T) {
	b := setup(t, config.ProcDirect, "einmal")
	b.cfg.Passes = 1
	var retries []int
	if _, err := Title(context.Background(), b.cfg, b.rel, Hooks{Retry: func(p, try int) { retries = append(retries, try) }}); err != nil {
		t.Fatal(err)
	}
	if n := len(b.calls(t)); n != 2 || !reflect.DeepEqual(retries, []int{2}) {
		t.Fatalf("%d calls, retries %v", n, retries)
	}
	if got, _ := os.ReadFile(b.target); !bytes.Equal(got, b.data) {
		t.Fatal("the improved title is not bit-identical")
	}
	b.clean(t)
}

// Never bit-identical: give up after the retries, the title stays as it was.
func TestTitleDirectNotIdentical(t *testing.T) {
	b := setup(t, config.ProcDirect, "falsch")
	_, err := Title(context.Background(), b.cfg, b.rel, Hooks{})
	if !errors.Is(err, ErrNotIdentical) {
		t.Fatalf("want ErrNotIdentical, got %v", err)
	}
	if n := len(b.calls(t)); n != b.cfg.Retries {
		t.Fatalf("%d calls, want %d", n, b.cfg.Retries)
	}
	b.untouched(t)
}

// improvedirect fails (e.g. the drive refuses O_DIRECT): its message reaches
// the log, the title stays as it was.
func TestTitleDirectProgramFails(t *testing.T) {
	b := setup(t, config.ProcDirect, "fehler")
	_, err := Title(context.Background(), b.cfg, b.rel, Hooks{})
	if err == nil || !strings.Contains(err.Error(), "Cannot open output file") {
		t.Fatalf("want improvedirect's message, got %v", err)
	}
	b.untouched(t)
}

// The window ends (or playback starts) in the middle of a pass: the context
// is cancelled, improvedirect is killed, the title stays exactly as it was —
// in pass 1 it is improvedirect's INPUT and must survive the clean-up. The
// next night starts the title anew and finishes it.
func TestTitleDirectCancelled(t *testing.T) {
	for _, tc := range []struct {
		art  string
		pass string
	}{{"haengt", "1"}, {"zweiter", "2"}} {
		b := setup(t, config.ProcDirect, tc.art)
		ctx, cancel := context.WithCancelCause(context.Background())
		go func() {
			// Cancel once the hanging pass has begun to write.
			out := filepath.Join(b.cfg.WorkDir, "durchgang"+tc.pass)
			for i := 0; i < 1000; i++ {
				if _, err := os.Stat(out); err == nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			cancel(errors.New("outside_window"))
		}()
		start := time.Now()
		_, err := Title(ctx, b.cfg, b.rel, Hooks{})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("pass %s: want context.Canceled, got %v", tc.pass, err)
		}
		if d := time.Since(start); d > 20*time.Second {
			t.Fatalf("pass %s: the cancelled title took %s", tc.pass, d)
		}
		if n := len(b.calls(t)); strconv.Itoa(n) != tc.pass {
			t.Fatalf("pass %s: %d calls", tc.pass, n)
		}
		b.untouched(t)

		// Next night: same title, from the start.
		t.Setenv("ATTRAPPE_ART", "gut")
		os.Remove(b.log)
		if _, err := Title(context.Background(), b.cfg, b.rel, Hooks{}); err != nil {
			t.Fatal("next night:", err)
		}
		if first := b.calls(t)[0]; !strings.Contains(first, "-- "+b.target+" ") {
			t.Fatalf("the next night must start with the title itself: %s", first)
		}
		if got, _ := os.ReadFile(b.target); !bytes.Equal(got, b.data) {
			t.Fatal("the improved title is not bit-identical")
		}
		b.clean(t)
	}
}

// The old procedure is untouched: bufhrt still writes from the original in
// RAM, and its record carries the method name of 0.1.x.
func TestTitleBufhrtReadsFromRAM(t *testing.T) {
	b := setup(t, config.ProcBufhrt, "gut")
	e, err := Title(context.Background(), b.cfg, b.rel, Hooks{})
	if err != nil {
		t.Fatal(err)
	}
	calls := b.calls(t)
	if len(calls) != 2 || !strings.Contains(calls[0], "--file="+filepath.Join(b.cfg.RAMDir, "original")+" ") ||
		!strings.Contains(calls[1], "--file="+filepath.Join(b.cfg.WorkDir, "durchgang1")+" ") {
		t.Fatalf("calls: %q", calls)
	}
	if e.Method != "frankl-0.9.3/sehr-langsam/2x" || e.Procedure != config.ProcBufhrt || e.Params["buffer_size"] != int64(419430400) {
		t.Fatalf("entry: %+v", e)
	}
	if got, _ := os.ReadFile(b.target); !bytes.Equal(got, b.data) {
		t.Fatal("the improved title is not bit-identical")
	}
	b.clean(t)
}
