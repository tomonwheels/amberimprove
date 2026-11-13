//go:build !linux

package cache

// DirectOK cannot ask for O_DIRECT on non-Linux systems (development
// builds); improvedirect itself is a Linux program.
func DirectOK(dir string) error { return nil }
