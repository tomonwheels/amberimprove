//go:build unix

package manifest

import (
	"os"
	"syscall"
)

func chownLike(path string, st os.FileInfo) {
	if s, ok := st.Sys().(*syscall.Stat_t); ok {
		_ = os.Chown(path, int(s.Uid), int(s.Gid))
	}
}
