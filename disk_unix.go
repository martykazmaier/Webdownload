//go:build unix

package main

import (
	"golang.org/x/sys/unix"
)

func diskFree(path string) (int64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, err
	}
	bsize := uint64(st.Bsize)
	if st.Frsize > 0 {
		bsize = uint64(st.Frsize)
	}
	avail := st.Bavail * bsize
	if avail > 1<<63-1 {
		return 1<<63 - 1, nil
	}
	return int64(avail), nil
}
