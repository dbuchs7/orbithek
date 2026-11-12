//go:build !unix

package share

import "io/fs"

// Device answers 0: the system does not tell the device here, so a
// missing mount is caught only by the directory being gone.
func Device(fs.FileInfo) uint64 { return 0 }
