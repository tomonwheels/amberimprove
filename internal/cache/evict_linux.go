//go:build linux

// Package cache flushes a file to the medium and drops it from the page cache,
// so the next read really comes from the SSD (instead of Harald's
// unmount/remount, which is impossible while the player uses the SSD).
package cache

import (
	"os"

	"golang.org/x/sys/unix"
)

// Evict fsyncs path and advises the kernel to drop its cached pages.
func Evict(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return err
	}
	return unix.Fadvise(int(f.Fd()), 0, 0, unix.FADV_DONTNEED)
}
