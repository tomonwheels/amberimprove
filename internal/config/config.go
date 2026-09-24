// Package config loads the amberIMPROVE configuration (JSON).
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Config is the complete amberIMPROVE configuration.
type Config struct {
	// Music is the library root on the target medium (the SSD), e.g.
	// /mnt/Music/Music. Every file below it is a candidate.
	Music string `json:"music"`
	// WorkDir holds the intermediate pass files. It MUST be on the same
	// filesystem as Music (the final step is a rename) but outside Music, so
	// neither the library scanner nor the backup ever sees it.
	WorkDir string `json:"work_dir"`
	// RAMDir receives the original fetched from the source (tmpfs).
	RAMDir string `json:"ram_dir"`
	// StateDir keeps zustand.json (status for `status` / `serve`).
	StateDir string `json:"state_dir"`

	Source     Source   `json:"source"`
	Extensions []string `json:"extensions"`

	// Passes is the number of improvefile runs per title (>= 1).
	Passes int `json:"passes"`
	// Retries per pass when the written file is not bit-identical.
	Retries int `json:"retries"`
	// Preset is the chosen quality level (see Presets); "" = custom values.
	Preset string `json:"preset"`
	// Method identifies the procedure. Titles improved with another method
	// count as "older method" and are queued again.
	Method string `json:"method"`

	Bufhrt   Bufhrt   `json:"bufhrt"`
	Window   Window   `json:"window"`
	Playback Playback `json:"playback"`

	// StatusURL is where this status page is reachable (set by the setup page
	// from the address it was opened with). It is written into every album
	// record, so players can link back here. Empty = no link.
	StatusURL string `json:"status_url,omitempty"`

	// MtimeTolerance in seconds between SSD file and source original
	// (the SSD was vfat once: 2 s granularity).
	MtimeTolerance int `json:"mtime_tolerance_s"`
}

// Source describes where the untouched originals live.
type Source struct {
	// Kind is "self" (the file itself is the original: read into RAM,
	// verified bit by bit — for users without a second copy), "rsync"
	// (remote via ssh) or "path" (locally mounted second copy).
	Kind string `json:"kind"`
	// Remote is user@host:/path for Kind "rsync".
	Remote string `json:"remote"`
	// Program is the ssh client: "ssh" (OpenSSH, default) or "dbclient"
	// (dropbear, e.g. on minimal systems). Only editable in the file.
	Program string `json:"program,omitempty"`
	// Key and Port are the ssh settings offered in the setup page.
	Key  string `json:"key"`
	Port int    `json:"port"`
	// SSH holds extra ssh arguments. Only editable in the file, never via
	// the setup page (ssh options can run commands).
	SSH []string `json:"ssh"`
	// Path is the local root for Kind "path".
	Path string `json:"path"`
}

// Bufhrt holds the program path and the improvefile parameters.
type Bufhrt struct {
	Program string `json:"program"`
	// CPU pins bufhrt with taskset (-1 = no pinning).
	CPU int `json:"cpu"`
	// RTPrio runs bufhrt with chrt -f (0 = normal scheduling).
	RTPrio int `json:"rt_prio"`

	BufferSize        int64 `json:"buffer_size"`
	LoopsPerSecond    int   `json:"loops_per_second"`
	BytesPerSecond    int64 `json:"bytes_per_second"`
	NumberCopies      int   `json:"number_copies"`
	RAMLoopsPerSecond int   `json:"ram_loops_per_second"`
	RAMBytesPerSecond int64 `json:"ram_bytes_per_second"`
	DsyncsPerSecond   int   `json:"dsyncs_per_second"`
	OutCopies         int   `json:"out_copies"`
	// Shift in ns (frankl, Jan 2025); 0 = option not passed.
	Shift int `json:"shift"`
}

// Window is the nightly time window plus blocked spans (local time, HH:MM).
type Window struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Blocked []Span `json:"blocked"`
}

// Span is a time span HH:MM–HH:MM, may cross midnight.
type Span struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Playback describes how to ask the player whether something is playing.
type Playback struct {
	// URL returns JSON; empty = no playback check.
	URL string `json:"url"`
	// Field is the JSON field holding the state.
	Field string `json:"field"`
	// Idle lists the values that mean "nothing is playing".
	Idle []string `json:"idle"`
	// UnreachableIsIdle: if the URL does not answer, may we work?
	// Default false = safe: no answer, no improving.
	UnreachableIsIdle bool `json:"unreachable_is_idle"`
}

// Default returns frankl's "very slow" improvefile parameters (the set found
// best in the forum tests, commented in frankl_stereo's scripts/improvefile,
// v0.9.3). It keeps perfect timing even on slow CPUs (0 delayed blocks on the
// NUC, measured 23.09.2026).
func Default() Config {
	c := Config{
		// Music and WorkDir stay empty until the setup page chose a drive.
		Music:      "",
		WorkDir:    "",
		RAMDir:     "/dev/shm/amberimprove",
		StateDir:   "/var/lib/amberimprove",
		Source:     Source{Kind: "self"},
		Extensions: []string{"flac", "wav", "aif", "aiff", "aifc", "dsf", "dff", "wv", "ape", "alac", "m4a", "mp3", "ogg", "opus"},
		Passes:     1,
		Retries:    10,
		Bufhrt:     Bufhrt{Program: "/opt/amberimprove/bin/bufhrt", CPU: -1},
		Window:     Window{From: "23:30", To: "06:30"},
		Playback: Playback{
			Field: "engine_state",
			Idle:  []string{"idle", "stopped", ""},
		},
		MtimeTolerance: 2,
	}
	if inContainer() {
		c.StateDir = "/config/state"
	}
	_ = c.ApplyPreset("sehr-langsam")
	return c
}

// Exists reports whether a configuration file is present.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Save validates c and writes it atomically.
func Save(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// inContainer: in a container the state lives next to the configuration
// (/config, the only persistent volume).
func inContainer() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil || os.Getenv("container") != ""
}

// Load reads path over the defaults and validates the result.
func Load(path string) (Config, error) {
	c := Default()
	b, err := os.ReadFile(path)
	if err != nil {
		return c, fmt.Errorf("Konfiguration %s nicht lesbar: %w", path, err)
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("Konfiguration %s fehlerhaft: %w", path, err)
	}
	// Values edited by hand no longer match the named preset: show "custom".
	if p, ok := FindPreset(c.Preset); !ok || !p.Bufhrt.sameParams(c.Bufhrt) {
		c.Preset = ""
	}
	return c, c.Validate()
}

// Validate checks the values that would otherwise fail late at night.
func (c Config) Validate() error {
	var errs []string
	if c.Music == "" {
		errs = append(errs, "bitte Laufwerk und Musikordner wählen")
	} else if !filepath.IsAbs(c.Music) {
		errs = append(errs, "music muss ein absoluter Pfad sein")
	}
	if !filepath.IsAbs(c.WorkDir) {
		errs = append(errs, "work_dir muss ein absoluter Pfad sein")
	} else if rel, err := filepath.Rel(c.Music, c.WorkDir); err == nil && !strings.HasPrefix(rel, "..") &&
		(rel == "." || !strings.HasPrefix(strings.Split(rel, string(filepath.Separator))[0], ".")) {
		// Inside music only as a hidden directory (the scanner skips those).
		errs = append(errs, "work_dir darf nicht innerhalb von music liegen (außer als versteckter Ordner)")
	}
	if !filepath.IsAbs(c.RAMDir) || !filepath.IsAbs(c.StateDir) {
		errs = append(errs, "ram_dir und state_dir müssen absolute Pfade sein")
	}
	switch c.Source.Kind {
	case "rsync":
		if !remoteRE.MatchString(c.Source.Remote) {
			errs = append(errs, "source.remote muss die Form benutzer@rechner:/pfad haben")
		}
		if c.Source.Key != "" && !filepath.IsAbs(c.Source.Key) {
			errs = append(errs, "source.key muss ein absoluter Pfad sein")
		}
	case "path":
		if !filepath.IsAbs(c.Source.Path) {
			errs = append(errs, "source.path muss ein absoluter Pfad sein")
		}
	case "self":
	default:
		errs = append(errs, `source.kind muss "self", "rsync" oder "path" sein`)
	}
	if c.StatusURL != "" && !statusRE.MatchString(c.StatusURL) {
		errs = append(errs, "status_url muss mit http:// oder https:// beginnen")
	}
	if c.Passes < 1 {
		errs = append(errs, "passes muss mindestens 1 sein")
	}
	if c.Retries < 1 {
		errs = append(errs, "retries muss mindestens 1 sein")
	}
	if c.Method == "" {
		errs = append(errs, "method fehlt")
	}
	if c.Bufhrt.BytesPerSecond <= 0 || c.Bufhrt.LoopsPerSecond <= 0 {
		errs = append(errs, "bufhrt.bytes_per_second und loops_per_second müssen > 0 sein")
	}
	for _, s := range append([]Span{{c.Window.From, c.Window.To}}, c.Window.Blocked...) {
		if _, err := ParseClock(s.From); err != nil {
			errs = append(errs, err.Error())
		}
		if _, err := ParseClock(s.To); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("Konfiguration: %s", strings.Join(errs, "; "))
	}
	return nil
}

var statusRE = regexp.MustCompile(`^https?://[A-Za-z0-9.\-\[\]:]+/?$`)

var remoteRE = regexp.MustCompile(`^[A-Za-z0-9._-]+@[A-Za-z0-9._-]+:/[^\x00]*$`)

// SSHProgram returns the ssh client to use.
func (s Source) SSHProgram() string {
	if s.Program == "" {
		return "ssh"
	}
	return s.Program
}

// SSHArgs builds the ssh arguments for the source.
func (s Source) SSHArgs() []string {
	a := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=20"}
	if filepath.Base(s.SSHProgram()) == "dbclient" {
		// dropbear knows no -o options; -y accepts a new host key once.
		a = []string{"-y"}
	}
	if s.Key != "" {
		a = append(a, "-i", s.Key)
	}
	if s.Port > 0 {
		a = append(a, "-p", strconv.Itoa(s.Port))
	}
	return append(a, s.SSH...)
}

// IsAudio reports whether name has one of the configured extensions and is
// not a macOS AppleDouble file.
func (c Config) IsAudio(name string) bool {
	if strings.HasPrefix(name, "._") || strings.HasPrefix(name, ".") {
		return false
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	for _, e := range c.Extensions {
		if ext == strings.ToLower(e) {
			return true
		}
	}
	return false
}

// ParseClock parses HH:MM into minutes after midnight.
func ParseClock(s string) (int, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("Uhrzeit %q ungültig (HH:MM)", s)
	}
	return t.Hour()*60 + t.Minute(), nil
}
