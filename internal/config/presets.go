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

// FindPreset returns the preset with id.
func FindPreset(id string) (Preset, bool) {
	for _, p := range Presets {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}

// ApplyPreset copies the preset's parameters into c (program, CPU and RT
// priority stay) and derives the method name from preset and passes.
func (c *Config) ApplyPreset(id string) error {
	p, ok := FindPreset(id)
	if !ok {
		return fmt.Errorf("unbekannte Qualitätsstufe %q", id)
	}
	prog, cpu, rt, shift := c.Bufhrt.Program, c.Bufhrt.CPU, c.Bufhrt.RTPrio, c.Bufhrt.Shift
	c.Bufhrt = p.Bufhrt
	c.Bufhrt.Program, c.Bufhrt.CPU, c.Bufhrt.RTPrio, c.Bufhrt.Shift = prog, cpu, rt, shift
	c.Preset = id
	c.Method = MethodName(id, c.Passes)
	return nil
}

// MethodName is the recorded procedure: frankl version, preset, passes.
func MethodName(preset string, passes int) string {
	return fmt.Sprintf("frankl-0.9.3/%s/%dx", preset, passes)
}

// sameParams compares the timing parameters (not program, CPU, priority).
func (b Bufhrt) sameParams(o Bufhrt) bool {
	return b.BufferSize == o.BufferSize && b.LoopsPerSecond == o.LoopsPerSecond &&
		b.BytesPerSecond == o.BytesPerSecond && b.NumberCopies == o.NumberCopies &&
		b.RAMLoopsPerSecond == o.RAMLoopsPerSecond && b.RAMBytesPerSecond == o.RAMBytesPerSecond &&
		b.DsyncsPerSecond == o.DsyncsPerSecond && b.OutCopies == o.OutCopies
}
