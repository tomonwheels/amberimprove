package config

import "fmt"

// Preset is a named bufhrt parameter set offered in the setup.
type Preset struct {
	ID     string `json:"id"`
	Bufhrt Bufhrt `json:"bufhrt"`
}

// Presets are ordered from best (slowest) to fastest. All three kept perfect
// timing (0 delayed blocks) on the NUC, i7-10710U capped at 1.1 GHz, measured
// 23.09.2026. Whether a machine keeps the timing is measured in the setup.
var Presets = []Preset{
	// frankl's "very slow" set, commented in scripts/improvefile (v0.9.3):
	// found best in the forum tests (Horst, Harald, frankl).
	{ID: "sehr-langsam", Bufhrt: Bufhrt{BufferSize: 419430400, LoopsPerSecond: 10, BytesPerSecond: 188044,
		NumberCopies: 1, RAMLoopsPerSecond: 100, RAMBytesPerSecond: 1880440, DsyncsPerSecond: 1, OutCopies: 256}},
	{ID: "langsam", Bufhrt: Bufhrt{BufferSize: 419430400, LoopsPerSecond: 100, BytesPerSecond: 819200,
		NumberCopies: 1, RAMLoopsPerSecond: 100, RAMBytesPerSecond: 8192000, DsyncsPerSecond: 1, OutCopies: 10}},
	{ID: "zuegig", Bufhrt: Bufhrt{BufferSize: 419430400, LoopsPerSecond: 256, BytesPerSecond: 2097152,
		NumberCopies: 1, RAMLoopsPerSecond: 256, RAMBytesPerSecond: 6291456, DsyncsPerSecond: 1, OutCopies: 10}},
}

// DirectPreset is a named improvedirect parameter set offered in the setup.
type DirectPreset struct {
	ID     string `json:"id"`
	Direct Direct `json:"direct"`
}

// DirectPresets holds the one set frankl documents for improvedirect, in
// scripts/improvefile (frankl_stereo v0.9.4, line 21, "very good but slow"):
//
//	improvedirect -v -o -i -w "$1" "$2" 10 40960 10 128
//
// 10 loops per second, 40960 bytes per second (one 4096-byte page per loop),
// fdatasync every 10 loops, 128 cleaned buffers; O_DIRECT for reading and
// writing, warm-up. The faster line next to it in the script names another
// program (improvetest) that is not in the repository: no preset from it
// (13.11.2026).
var DirectPresets = []DirectPreset{
	{ID: "sehr-langsam", Direct: Direct{LoopsPerSecond: 10, BytesPerSecond: 40960, SyncLoops: 10, OutCopies: 128,
		DirectWrite: true, DirectRead: true, Warmup: true}},
}

// frankl_stereo versions the two programs are built from, and the segment
// that marks the direct procedure in a method name.
const (
	FranklBufhrt = "0.9.3"
	FranklDirect = "0.9.4"
	DirectTag    = "direkt"
)

// FindPreset returns the bufhrt preset with id.
func FindPreset(id string) (Preset, bool) {
	for _, p := range Presets {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}

// FindDirectPreset returns the improvedirect preset with id.
func FindDirectPreset(id string) (DirectPreset, bool) {
	for _, p := range DirectPresets {
		if p.ID == id {
			return p, true
		}
	}
	return DirectPreset{}, false
}

// ApplyPreset copies the parameters of preset id of the chosen procedure
// into c (program, CPU and RT priority stay) and derives the method name
// from procedure, preset and passes.
func (c *Config) ApplyPreset(id string) error {
	if c.IsDirect() {
		p, ok := FindDirectPreset(id)
		if !ok {
			return fmt.Errorf("unbekannte Qualitätsstufe %q für das Verfahren „direkt“", id)
		}
		c.Direct.setParams(p.Direct)
	} else {
		p, ok := FindPreset(id)
		if !ok {
			return fmt.Errorf("unbekannte Qualitätsstufe %q", id)
		}
		prog, cpu, rt, shift := c.Bufhrt.Program, c.Bufhrt.CPU, c.Bufhrt.RTPrio, c.Bufhrt.Shift
		c.Bufhrt = p.Bufhrt
		c.Bufhrt.Program, c.Bufhrt.CPU, c.Bufhrt.RTPrio, c.Bufhrt.Shift = prog, cpu, rt, shift
	}
	c.Preset = id
	c.Method = MethodName(c.Procedure, id, c.Passes)
	return nil
}

// MethodName is the recorded procedure: frankl version, (direct procedure,)
// preset, passes. The bufhrt names are those of 0.1.x, so titles improved
// before 0.2.0 stay "done".
func MethodName(procedure, preset string, passes int) string {
	if procedure == ProcDirect {
		return fmt.Sprintf("frankl-%s/%s/%s/%dx", FranklDirect, DirectTag, preset, passes)
	}
	return fmt.Sprintf("frankl-%s/%s/%dx", FranklBufhrt, preset, passes)
}

// presetMatches reports whether the named preset of the chosen procedure
// still has exactly the configured parameters.
func (c Config) presetMatches() bool {
	if c.IsDirect() {
		p, ok := FindDirectPreset(c.Preset)
		return ok && p.Direct.sameParams(c.Direct)
	}
	p, ok := FindPreset(c.Preset)
	return ok && p.Bufhrt.sameParams(c.Bufhrt)
}

// sameParams compares the timing parameters (not program, CPU, priority).
func (b Bufhrt) sameParams(o Bufhrt) bool {
	return b.BufferSize == o.BufferSize && b.LoopsPerSecond == o.LoopsPerSecond &&
		b.BytesPerSecond == o.BytesPerSecond && b.NumberCopies == o.NumberCopies &&
		b.RAMLoopsPerSecond == o.RAMLoopsPerSecond && b.RAMBytesPerSecond == o.RAMBytesPerSecond &&
		b.DsyncsPerSecond == o.DsyncsPerSecond && b.OutCopies == o.OutCopies
}

// setParams copies the parameters of p (program, CPU and priority stay).
func (d *Direct) setParams(p Direct) {
	p.Program, p.CPU, p.RTPrio = d.Program, d.CPU, d.RTPrio
	*d = p
}

// sameParams compares the parameters (not program, CPU, priority).
func (d Direct) sameParams(o Direct) bool {
	o.Program, o.CPU, o.RTPrio = d.Program, d.CPU, d.RTPrio
	return d == o
}
