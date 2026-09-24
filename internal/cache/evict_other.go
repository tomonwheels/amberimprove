//go:build !linux

package cache

import "os"

// Evict only fsyncs on non-Linux systems (development builds); the real
// cache drop needs posix_fadvise on Linux.
func Evict(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
