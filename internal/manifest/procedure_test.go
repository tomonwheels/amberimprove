package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The record names the procedure and its parameters; records written before
// 0.2.0 (no "procedure") still read, and switching the procedure makes a
// title count as "older method".
func TestProcedureInRecord(t *testing.T) {
	dir := t.TempDir()
	mt := time.Date(2024, 5, 2, 10, 11, 12, 0, time.UTC)
	m, _ := Read(dir)
	m.Files["01.flac"] = Entry{Size: 100, Mtime: mt, Method: "frankl-0.9.4/direkt/sehr-langsam/1x", Procedure: "direct",
		Passes: 1, Params: map[string]any{"out_copies": 128, "direct_read": true}, Delayed: 3}
	// As 0.1.x wrote it.
	m.Files["02.flac"] = Entry{Size: 100, Mtime: mt, Method: "frankl-0.9.3/sehr-langsam/1x", Passes: 1,
		Params: map[string]any{"out_copies": 256}}
	if err := Write(dir, m); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(raw), `"procedure"`); n != 1 || !strings.Contains(string(raw), `"procedure": "direct"`) {
		t.Fatalf("procedure must be written for the direct title only (%d):\n%s", n, raw)
	}
	got, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	e := got.Files["01.flac"]
	if e.Procedure != "direct" || e.Params["out_copies"] != float64(128) || e.Params["direct_read"] != true || e.Delayed != 3 {
		t.Fatalf("direct record: %+v", e)
	}
	if old := got.Files["02.flac"]; old.Procedure != "" || old.Params["out_copies"] != float64(256) {
		t.Fatalf("old record: %+v", old)
	}
	if s := got.StatusOf("01.flac", 100, mt, "frankl-0.9.4/direkt/sehr-langsam/1x"); s != Done {
		t.Fatalf("want Done, got %v", s)
	}
	// The user switched from bufhrt to direct: the bufhrt title is queued again.
	if s := got.StatusOf("02.flac", 100, mt, "frankl-0.9.4/direkt/sehr-langsam/1x"); s != Older {
		t.Fatalf("want Older, got %v", s)
	}
}
