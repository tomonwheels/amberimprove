package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// direct returns a valid configuration with the direct procedure and
// frankl's set.
func direct(t *testing.T) Config {
	t.Helper()
	c := Default()
	c.Music, c.WorkDir = "/mnt/Music/Music", "/mnt/Music/.amberimprove-tmp"
	c.Procedure = ProcDirect
	if err := c.ApplyPreset("sehr-langsam"); err != nil {
		t.Fatal(err)
	}
	return c
}

// The old procedure stays the default (Tom's ear decides later), and its
// method name stays the one of 0.1.x — otherwise every improved title would
// be queued again after the update.
func TestDefaultStaysBufhrt(t *testing.T) {
	c := Default()
	if c.Procedure != ProcBufhrt || c.IsDirect() {
		t.Fatalf("default procedure: %q", c.Procedure)
	}
	if c.Method != "frankl-0.9.3/sehr-langsam/1x" {
		t.Fatal("default method:", c.Method)
	}
	if c.WriteRate() != 188044 || c.Overhead() != 1.5 {
		t.Fatalf("default rate %d overhead %v", c.WriteRate(), c.Overhead())
	}
	// The direct block is ready to use without copying numbers.
	if c.Direct.Program != "/opt/amberimprove/bin/improvedirect" || c.Direct.CPU != -1 {
		t.Fatalf("direct program/cpu: %+v", c.Direct)
	}
}

// frankl's line in scripts/improvefile (v0.9.4):
// improvedirect -v -o -i -w "$1" "$2" 10 40960 10 128
func TestDirectPresetIsFrankls(t *testing.T) {
	c := direct(t)
	d := c.Direct
	if d.LoopsPerSecond != 10 || d.BytesPerSecond != 40960 || d.SyncLoops != 10 || d.OutCopies != 128 {
		t.Fatalf("numbers: %+v", d)
	}
	if !d.DirectWrite || !d.DirectRead || !d.Warmup || d.Dsync || d.Preallocate || d.Align != 0 {
		t.Fatalf("switches: %+v", d)
	}
	if d.ChunkSize() != 4096 || d.Rate() != 40960 || c.WriteRate() != 40960 {
		t.Fatalf("chunk %d rate %d", d.ChunkSize(), d.Rate())
	}
	if c.Method != "frankl-0.9.4/direkt/sehr-langsam/1x" || c.Preset != "sehr-langsam" {
		t.Fatalf("method %q preset %q", c.Method, c.Preset)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	// Program and pinning survive a preset.
	c.Direct.Program, c.Direct.CPU, c.Direct.RTPrio = "/x/improvedirect", 3, 60
	c.Passes = 2
	if err := c.ApplyPreset("sehr-langsam"); err != nil {
		t.Fatal(err)
	}
	if c.Direct.Program != "/x/improvedirect" || c.Direct.CPU != 3 || c.Direct.RTPrio != 60 {
		t.Fatalf("program/pinning lost: %+v", c.Direct)
	}
	if c.Method != "frankl-0.9.4/direkt/sehr-langsam/2x" {
		t.Fatal("method:", c.Method)
	}
	// The bufhrt presets do not exist for the direct procedure.
	if err := c.ApplyPreset("zuegig"); err == nil {
		t.Fatal("bufhrt preset must be rejected for the direct procedure")
	}
}

// The chunk is computed like improvedirect does: bps/lps rounded down to the
// alignment (default 4096 with -o or -i, otherwise 1).
func TestDirectChunk(t *testing.T) {
	for _, tc := range []struct {
		d     Direct
		chunk int64
		rate  int64
	}{
		{Direct{LoopsPerSecond: 10, BytesPerSecond: 40960, DirectWrite: true}, 4096, 40960},
		{Direct{LoopsPerSecond: 10, BytesPerSecond: 50000, DirectRead: true}, 4096, 40960},
		{Direct{LoopsPerSecond: 10, BytesPerSecond: 50000}, 5000, 50000},
		{Direct{LoopsPerSecond: 10, BytesPerSecond: 50000, DirectWrite: true, Align: 512}, 4608, 46080},
		{Direct{LoopsPerSecond: 100, BytesPerSecond: 40960, DirectWrite: true}, 0, 0},
		{Direct{}, 0, 0},
	} {
		if got := tc.d.ChunkSize(); got != tc.chunk {
			t.Errorf("%+v: chunk %d, want %d", tc.d, got, tc.chunk)
		}
		if got := tc.d.Rate(); got != tc.rate {
			t.Errorf("%+v: rate %d, want %d", tc.d, got, tc.rate)
		}
	}
}

func TestValidateDirect(t *testing.T) {
	ok := direct(t)
	for name, change := range map[string]func(*Config){
		"unknown procedure":        func(c *Config) { c.Procedure = "schnell" },
		"empty procedure":          func(c *Config) { c.Procedure = "" },
		"bufhrt method for direct": func(c *Config) { c.Method = "frankl-0.9.3/sehr-langsam/1x" },
		"direct method for bufhrt": func(c *Config) { c.Procedure = ProcBufhrt },
		"too many copies":          func(c *Config) { c.Direct.OutCopies = 1001 },
		"no sync":                  func(c *Config) { c.Direct.SyncLoops = 0 },
		"chunk below one page":     func(c *Config) { c.Direct.LoopsPerSecond = 100 },
		"alignment not 512":        func(c *Config) { c.Direct.Align = 1000 },
		"relative program":         func(c *Config) { c.Direct.Program = "improvedirect" },
	} {
		c := ok
		change(&c)
		if c.Validate() == nil {
			t.Errorf("%s: must be rejected", name)
		}
	}
	// Custom values with their own method name are fine.
	c := ok
	c.Direct.OutCopies, c.Preset = 64, ""
	c.Method = MethodName(ProcDirect, "eigen", c.Passes)
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if c.Method != "frankl-0.9.4/direkt/eigen/1x" {
		t.Fatal("method:", c.Method)
	}
}

func TestSaveLoadDirect(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	c := direct(t)
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Procedure != ProcDirect || got.Preset != "sehr-langsam" || got.Direct != c.Direct || got.Method != c.Method {
		t.Fatalf("round trip: %+v", got)
	}
	// Values changed by hand no longer carry the preset's name.
	c.Direct.OutCopies = 64
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	if got, err = Load(path); err != nil || got.Preset != "" {
		t.Fatalf("hand-edited values must show as custom: preset %q, %v", got.Preset, err)
	}
}

// A configuration written by 0.1.x has neither "procedure" nor "direct": it
// must load unchanged as bufhrt.
func TestLoadOldConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	old := `{"music": "/mnt/Music/Music", "work_dir": "/mnt/Music/.amberimprove-tmp",
		"source": {"kind": "self"}, "passes": 1, "retries": 10,
		"preset": "sehr-langsam", "method": "frankl-0.9.3/sehr-langsam/1x",
		"bufhrt": {"program": "/opt/amberimprove/bin/bufhrt", "cpu": -1, "buffer_size": 419430400,
			"loops_per_second": 10, "bytes_per_second": 188044, "number_copies": 1,
			"ram_loops_per_second": 100, "ram_bytes_per_second": 1880440,
			"dsyncs_per_second": 1, "out_copies": 256},
		"window": {"from": "23:30", "to": "06:30"}}`
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Procedure != ProcBufhrt || c.Preset != "sehr-langsam" || c.Method != "frankl-0.9.3/sehr-langsam/1x" {
		t.Fatalf("old config: procedure %q preset %q method %q", c.Procedure, c.Preset, c.Method)
	}
	if !strings.HasSuffix(c.Direct.Program, "/improvedirect") || c.Direct.OutCopies != 128 {
		t.Fatalf("direct defaults missing: %+v", c.Direct)
	}
}

// The example file shows both blocks; switching it to "direct" needs only
// the two lines procedure and method.
func TestExampleConfig(t *testing.T) {
	c, err := Load("../../deploy/config.example.json")
	if err != nil {
		t.Fatal(err)
	}
	if c.Procedure != ProcBufhrt || c.Preset != "sehr-langsam" {
		t.Fatalf("example: procedure %q preset %q", c.Procedure, c.Preset)
	}
	if want := DirectPresets[0].Direct; !want.sameParams(c.Direct) {
		t.Fatalf("example direct block is not frankl's set: %+v", c.Direct)
	}
	c.Procedure, c.Method = ProcDirect, MethodName(ProcDirect, "sehr-langsam", c.Passes)
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}
