package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tomonwheels/amberimprove/internal/config"
	"github.com/tomonwheels/amberimprove/internal/guard"
	"github.com/tomonwheels/amberimprove/internal/manifest"
	"github.com/tomonwheels/amberimprove/internal/queue"
	"github.com/tomonwheels/amberimprove/internal/state"
)

// Stand-in for frankl's improvedirect (a Linux program): copies like the
// real one; with ATTRAPPE_ART=haengt it writes a part and then hangs until
// it is killed — a pass that is still running when the window ends.
const directAttrappe = `#!/bin/sh
while [ "$1" != "--" ]; do shift; done
in="$2"; out="$3"
if [ "$ATTRAPPE_ART" = haengt ]; then head -c 10 "$in" > "$out"; exec sleep 60; fi
cat "$in" > "$out"
echo "improvedirect: 5 bytes written, 1 fdatasync, timer reset 0 times" >&2
`

// directConfig is a configuration with the direct procedure, frankl's set
// and the stand-in program, on a music folder with one title.
func directConfig(t *testing.T) (config.Config, string, []byte) {
	t.Helper()
	dir := t.TempDir()
	c := config.Default()
	c.Music = filepath.Join(dir, "musik")
	c.WorkDir = filepath.Join(dir, ".amberimprove-tmp")
	c.RAMDir = filepath.Join(dir, "ram")
	c.StateDir = filepath.Join(dir, "zustand")
	c.Procedure = config.ProcDirect
	if err := c.ApplyPreset("sehr-langsam"); err != nil {
		t.Fatal(err)
	}
	c.Direct.Program = filepath.Join(dir, "improvedirect")
	if err := os.WriteFile(c.Direct.Program, []byte(directAttrappe), 0o755); err != nil {
		t.Fatal(err)
	}
	rel := "Album/01 Titel.flac"
	data := bytes.Repeat([]byte("amberIMPROVE Probe\n"), 5000)
	if err := os.MkdirAll(filepath.Join(c.Music, "Album"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.Music, rel), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	return c, rel, data
}

func clock(t time.Time) string { return t.Format("15:04") }

// The time estimate decides which title still fits into the window: it must
// use the rate of the chosen procedure. improvedirect with frankl's set
// writes 40960 bytes per second — a title takes far longer than with bufhrt.
func TestEstimateByProcedure(t *testing.T) {
	const size = 40960 * 1000 // about 39 MB
	c := config.Default()
	// 40 960 000 bytes / 188 044 bytes per second × 1.5 = 326.7 s.
	if got := estimate(c, size).Round(time.Second); got != 327*time.Second {
		t.Fatalf("bufhrt: %s, want 327 s", got)
	}
	c.Procedure = config.ProcDirect
	if err := c.ApplyPreset("sehr-langsam"); err != nil {
		t.Fatal(err)
	}
	if got, want := estimate(c, size).Round(time.Second), 1200*time.Second; got != want {
		t.Fatalf("direct: %s, want %s (1000 s writing × 1.2)", got, want)
	}
	c.Passes = 3
	if got, want := estimate(c, size).Round(time.Second), 3600*time.Second; got != want {
		t.Fatalf("direct, 3 passes: %s, want %s", got, want)
	}
}

// The setup page may choose procedure and preset; the program paths never
// come from the browser, and the method name follows the procedure.
func TestCandidate(t *testing.T) {
	cur := config.Default()
	cur.Music, cur.WorkDir = "/mnt/Music/Music", "/mnt/Music/.amberimprove-tmp"
	c, err := candidate(cur, strings.NewReader(`{"procedure":"direct","preset":"sehr-langsam","passes":2,
		"direct":{"program":"/tmp/boese","out_copies":7},"bufhrt":{"program":"/tmp/boese"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Direct.Program != cur.Direct.Program || c.Bufhrt.Program != cur.Bufhrt.Program {
		t.Fatalf("program path taken from the browser: %q %q", c.Direct.Program, c.Bufhrt.Program)
	}
	if c.Method != "frankl-0.9.4/direkt/sehr-langsam/2x" || c.Direct.OutCopies != 128 || !c.Direct.DirectRead {
		t.Fatalf("method %q, direct %+v", c.Method, c.Direct)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	// A preset of the other procedure is refused.
	if _, err := candidate(cur, strings.NewReader(`{"procedure":"direct","preset":"zuegig"}`)); err == nil {
		t.Fatal("bufhrt preset with the direct procedure must be refused")
	}
	// Custom values: changing the procedure must not keep the old method name.
	c, err = candidate(cur, strings.NewReader(`{"procedure":"direct","preset":""}`))
	if err != nil || c.Method != "frankl-0.9.4/direkt/eigen/1x" {
		t.Fatalf("custom direct: method %q, %v", c.Method, err)
	}
	// And back: the page as 0.1.x sent it (no procedure) stays bufhrt.
	c, err = candidate(cur, strings.NewReader(`{"preset":"langsam"}`))
	if err != nil || c.Procedure != config.ProcBufhrt || c.Method != "frankl-0.9.3/langsam/1x" {
		t.Fatalf("bufhrt: procedure %q method %q, %v", c.Procedure, c.Method, err)
	}
}

// The window ends while improvedirect is writing: the guard (unchanged, the
// same watch as for bufhrt) cancels the title, the program is killed, the
// title on the medium is untouched and still open — and the next night
// improves it from the start.
func TestWindowEndsDuringDirectPass(t *testing.T) {
	watchInterval = 50 * time.Millisecond
	defer func() { watchInterval = 10 * time.Second }()

	cfg, rel, data := directConfig(t)
	target := filepath.Join(cfg.Music, rel)
	before, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	st, err := state.Open(cfg.StateDir)
	if err != nil {
		t.Fatal(err)
	}
	title := queue.Title{Rel: rel, Size: int64(len(data))}

	// Night 1: the window is over (it lies two hours ahead) while the pass
	// still runs.
	t.Setenv("ATTRAPPE_ART", "haengt")
	over := cfg
	now := time.Now()
	over.Window = config.Window{From: clock(now.Add(2 * time.Hour)), To: clock(now.Add(3 * time.Hour))}
	if guard.InWindow(over.Window, now) {
		t.Fatal("test window must be closed")
	}
	fctx, fcancel := context.WithCancelCause(context.Background())
	go watch(fctx, over, fcancel)
	start := time.Now()
	err = processOne(fctx, cfg, st, title)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if why := context.Cause(fctx); why == nil || why.Error() != guard.ReasonOutside {
		t.Fatalf("cause: %v", why)
	}
	fcancel(nil)
	if d := time.Since(start); d > 20*time.Second {
		t.Fatalf("stopping took %s", d)
	}
	after, err := os.Stat(target)
	if err != nil || !os.SameFile(before, after) || !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("the title was touched: %v", err)
	}
	if got, _ := os.ReadFile(target); !bytes.Equal(got, data) {
		t.Fatal("the title's content changed")
	}
	if entries, _ := os.ReadDir(cfg.WorkDir); len(entries) != 0 {
		t.Fatalf("pass files left behind: %v", entries)
	}
	s, _ := state.Read(cfg.StateDir)
	if last := s.Events[len(s.Events)-1]; last.Kind != "interrupted" || last.Detail != guard.ReasonOutside || s.Done != 0 || s.Failed != 0 {
		t.Fatalf("state: last event %+v, done %d, failed %d", last, s.Done, s.Failed)
	}
	if _, err := os.Stat(filepath.Join(cfg.Music, "Album", manifest.FileName)); err == nil {
		t.Fatal("an interrupted title must not be recorded")
	}
	titles, _, err := queue.Scan(cfg)
	if err != nil || len(queue.Pending(titles)) != 1 {
		t.Fatalf("the title must still be open: %v %v", titles, err)
	}

	// Night 2: the window is open, the pass runs through.
	t.Setenv("ATTRAPPE_ART", "gut")
	open := cfg
	open.Window = config.Window{From: "00:00", To: "00:00"}
	fctx, fcancel = context.WithCancelCause(context.Background())
	go watch(fctx, open, fcancel)
	err = processOne(fctx, cfg, st, title)
	fcancel(nil)
	if err != nil {
		t.Fatal("night 2:", err)
	}
	if got, _ := os.ReadFile(target); !bytes.Equal(got, data) {
		t.Fatal("the improved title is not bit-identical")
	}
	m, err := manifest.Read(filepath.Join(cfg.Music, "Album"))
	if err != nil {
		t.Fatal(err)
	}
	if e := m.Files["01 Titel.flac"]; e.Procedure != config.ProcDirect || e.Method != cfg.Method {
		t.Fatalf("record: %+v", e)
	}
	titles, _, err = queue.Scan(cfg)
	if err != nil || len(queue.Pending(titles)) != 0 {
		t.Fatalf("the title must be done: %v %v", titles, err)
	}
}
