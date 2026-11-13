//go:build linux

package cache

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// DirectOK reports whether the filesystem holding dir lets a file be opened
// with O_DIRECT — what improvedirect does with -o and -i. RAM disks before
// Linux 6.6 and some FUSE or overlay mounts refuse it (EINVAL); then every
// title would fail at night, so the setup asks first (13.11.2026). It
// creates an empty hidden file in dir and removes it again.
func DirectOK(dir string) error {
	f, err := os.CreateTemp(dir, ".amberimprove-direktprobe-*")
	if err != nil {
		return err
	}
	name := f.Name()
	f.Close()
	defer os.Remove(name)
	fd, err := unix.Open(name, unix.O_WRONLY|unix.O_DIRECT, 0)
	if err != nil {
		return fmt.Errorf("O_DIRECT: %w", err)
	}
	return unix.Close(fd)
}
