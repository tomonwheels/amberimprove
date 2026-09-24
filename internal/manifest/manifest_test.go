package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStatusAndRoundTrip(t *testing.T) {
	dir := t.TempDir()
	mt := time.Date(2024, 5, 2, 10, 11, 12, 123456789, time.UTC)
	m, err := Read(dir)
	if err != nil || len(m.Files) != 0 {
		t.Fatal("empty manifest expected")
	}
	m.Files["01.flac"] = Entry{Size: 100, Mtime: mt, Method: "m1", Passes: 3}
	if err := Write(dir, m); err != nil {
		t.Fatal(err)
	}
	m2, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s := m2.StatusOf("01.flac", 100, mt, "m1"); s != Done {
		t.Fatalf("want Done, got %v", s)
	}
	if s := m2.StatusOf("01.flac", 100, mt, "m2"); s != Older {
		t.Fatalf("want Older, got %v", s)
	}
	if s := m2.StatusOf("01.flac", 101, mt, "m1"); s != Open {
		t.Fatal("changed size must reopen the title")
	}
	if s := m2.StatusOf("01.flac", 100, mt.Add(time.Second), "m1"); s != Open {
		t.Fatal("changed mtime must reopen the title")
	}
	if s := m2.StatusOf("02.flac", 100, mt, "m1"); s != Open {
		t.Fatal("unknown title must be open")
	}
}

func TestWriteStatus(t *testing.T) {
	dir := t.TempDir()
	if err := WriteStatus(dir, "", "m"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, StatusFileName)); err == nil {
		t.Fatal("without status_url nothing may be written")
	}
	if err := WriteStatus(dir, "http://nuc:8093", "m1"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, StatusFileName))
	if err != nil || !strings.Contains(string(b), "http://nuc:8093") {
		t.Fatalf("status file: %s %v", b, err)
	}
}
