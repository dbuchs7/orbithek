//go:build unix

package share

import (
	"io/fs"
	"syscall"
)

// Device answers the device a file lies on.
func Device(fi fs.FileInfo) uint64 {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Dev)
	}
	return 0
}
