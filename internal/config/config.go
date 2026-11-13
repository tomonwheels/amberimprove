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
	// Procedure chooses the program that writes a pass: ProcBufhrt (original
	// in RAM, written by bufhrt --interval; the default) or ProcDirect
	// (frankl's improvedirect: reads and writes small blocks in turns,
	// straight from and to the medium). Added 13.11.2026 (0.2.0); files
	// written by 0.1.x have no such field and stay on bufhrt.
	Procedure string `json:"procedure"`

	Bufhrt   Bufhrt   `json:"bufhrt"`
	Direct   Direct   `json:"direct"`
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

// Procedures (values of Config.Procedure).
const (
	ProcBufhrt = "bufhrt"
	ProcDirect = "direct"
)

// Direct holds the program path and the parameters of frankl's improvedirect
// (frankl_stereo v0.9.4, src/improvedirect.c):
//
//	improvedirect [options] origfile copiedfile lps bps nsync ncp
//
// The field comments follow the program's own help text.
type Direct struct {
	Program string `json:"program"`
	// CPU pins improvedirect with taskset (-1 = no pinning); frankl:
	// "If applicable start the above on an isolated CPU".
	CPU int `json:"cpu"`
	// RTPrio runs improvedirect with chrt -f (0 = normal scheduling).
	RTPrio int `json:"rt_prio"`

	// LoopsPerSecond (lps) and BytesPerSecond (bps): one chunk of bps/lps
	// bytes is read and written per loop.
	LoopsPerSecond int   `json:"loops_per_second"`
	BytesPerSecond int64 `json:"bytes_per_second"`
	// SyncLoops (nsync): fdatasync every nsync loops.
	SyncLoops int `json:"sync_loops"`
	// OutCopies (ncp): each chunk passes through ncp cleaned buffers before
	// it is written (1..1000).
	OutCopies int `json:"out_copies"`
	// DirectWrite (-o): write with O_DIRECT, not into the page cache.
	DirectWrite bool `json:"direct_write"`
	// DirectRead (-i): read with O_DIRECT (no page cache).
	DirectRead bool `json:"direct_read"`
	// Dsync (-y): write with O_DSYNC.
	Dsync bool `json:"dsync"`
	// Warmup (-w): a cheap system call directly after the sleep.
	Warmup bool `json:"warmup"`
	// Preallocate (-p): preallocate the output file with fallocate.
	Preallocate bool `json:"preallocate"`
	// Align (-a): the chunk is rounded down to a multiple of Align bytes;
	// 0 = the program's default (4096 with -o or -i, otherwise 1).
	Align int `json:"align"`
}

// EffectiveAlign is the alignment improvedirect uses (its default for 0).
func (d Direct) EffectiveAlign() int64 {
	if d.Align > 0 {
		return int64(d.Align)
	}
	if d.DirectWrite || d.DirectRead {
		return 4096
	}
	return 1
}

// ChunkSize is the number of bytes per loop, computed like improvedirect
// does (bps/lps rounded down to the alignment); 0 = invalid parameters.
func (d Direct) ChunkSize() int64 {
	if d.LoopsPerSecond < 1 || d.BytesPerSecond < 1 {
		return 0
	}
	n := d.BytesPerSecond / int64(d.LoopsPerSecond)
	return n - n%d.EffectiveAlign()
}

// Rate is the real writing rate in bytes per second (chunk × loops). It can
// be lower than BytesPerSecond because of the rounding.
func (d Direct) Rate() int64 { return d.ChunkSize() * int64(d.LoopsPerSecond) }

// IsDirect reports whether passes are written by improvedirect.
func (c Config) IsDirect() bool { return c.Procedure == ProcDirect }

// WriteRate is the writing rate of one pass in bytes per second — the one
// place every time estimate asks (night run, plan, setup, status page).
func (c Config) WriteRate() int64 {
	if c.IsDirect() {
		return c.Direct.Rate()
	}
	return c.Bufhrt.BytesPerSecond
}

// Overhead is the factor on top of the pure writing time of a title.
// bufhrt: RAM copies and verification, measured on the NUC in Sept 2026
// (1.43, rounded up). improvedirect has no RAM phase; what comes on top is
// the verification read and the timer resets after slow fdatasync calls.
// Its 1.2 is DERIVED from the source (13.11.2026), not yet measured on a
// machine — measure and correct it here.
func (c Config) Overhead() float64 {
	if c.IsDirect() {
		return 1.2
	}
	return 1.5
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
		Procedure:  ProcBufhrt,
		Bufhrt:     Bufhrt{Program: "/opt/amberimprove/bin/bufhrt", CPU: -1},
		Direct:     Direct{Program: "/opt/amberimprove/bin/improvedirect", CPU: -1},
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
	// The direct block carries frankl's set from the start, so choosing
	// "direct" by hand in the file works without copying numbers.
	c.Direct.setParams(DirectPresets[0].Direct)
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
	if !c.presetMatches() {
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
	switch c.Procedure {
	case ProcBufhrt:
		// The method name is what the album records compare: a bufhrt
		// configuration must not carry the name of the direct procedure.
		if strings.Contains(c.Method, "/"+DirectTag+"/") {
			errs = append(errs, `method nennt das Verfahren „direkt“, procedure ist aber "bufhrt"`)
		}
	case ProcDirect:
		d := c.Direct
		// The same limits improvedirect checks itself ("invalid arguments",
		// "chunk size … is smaller than …") — found here, not at night.
		if d.LoopsPerSecond < 1 || d.BytesPerSecond < 1 || d.SyncLoops < 1 || d.OutCopies < 1 || d.OutCopies > 1000 {
			errs = append(errs, "direct: loops_per_second, bytes_per_second und sync_loops müssen >= 1 sein, out_copies 1 bis 1000")
		} else if d.ChunkSize() <= 0 {
			errs = append(errs, fmt.Sprintf("direct: bytes_per_second/loops_per_second ist kleiner als ein Block (%d Bytes)", d.EffectiveAlign()))
		}
		if d.Align < 0 || ((d.DirectWrite || d.DirectRead) && d.EffectiveAlign()%512 != 0) {
			errs = append(errs, "direct.align muss mit direct_write/direct_read ein Vielfaches von 512 sein")
		}
		if !filepath.IsAbs(d.Program) {
			errs = append(errs, "direct.program muss ein absoluter Pfad sein")
		}
		if c.Method != "" && !strings.Contains(c.Method, "/"+DirectTag+"/") {
			errs = append(errs, `method muss bei procedure "direct" das Verfahren nennen, z. B. `+MethodName(ProcDirect, "eigen", c.Passes))
		}
	default:
		errs = append(errs, `procedure muss "bufhrt" oder "direct" sein`)
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
