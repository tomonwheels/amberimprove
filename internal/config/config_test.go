package config

import "testing"

func TestValidate(t *testing.T) {
	if Default().Validate() == nil {
		t.Fatal("a fresh default without music folder must not validate")
	}
	c := Default()
	c.Music, c.WorkDir = "/mnt/Music/Music", "/mnt/Music/.amberimprove-tmp"
	c.Source = Source{Kind: "path", Path: "/srv/originals"}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := c
	bad.WorkDir = "/mnt/Music/Music/tmp"
	if bad.Validate() == nil {
		t.Fatal("visible work_dir inside music must be rejected")
	}
	hidden := c
	hidden.WorkDir = "/mnt/Music/Music/.amberimprove-tmp"
	if err := hidden.Validate(); err != nil {
		t.Fatal("hidden work_dir inside music must be allowed:", err)
	}
	if Default().Method != "frankl-0.9.3/sehr-langsam/1x" {
		t.Fatal("default method:", Default().Method)
	}
	bad = c
	bad.Window.From = "25:00"
	if bad.Validate() == nil {
		t.Fatal("invalid clock must be rejected")
	}
}

func TestIsAudio(t *testing.T) {
	c := Default()
	for name, want := range map[string]bool{"01 Titel.FLAC": true, "._01 Titel.flac": false, "cover.jpg": false, ".amberimprove.json": false, "a.dsf": true} {
		if got := c.IsAudio(name); got != want {
			t.Errorf("IsAudio(%q) = %v", name, got)
		}
	}
}
