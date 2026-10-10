//go:build !windows && !unix

package main

import (
	"fmt"
	"runtime"
)

func diskFree(path string) (int64, error) {
	return 0, fmt.Errorf("free space check is not supported on %s", runtime.GOOS)
}
