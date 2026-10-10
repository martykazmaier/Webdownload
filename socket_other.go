//go:build !windows && !unix

package main

import (
	"fmt"
	"net"
	"runtime"
)

func connFromHandle(handle uintptr) (net.Conn, error) {
	return nil, fmt.Errorf("inherited socket handle %d is not supported on %s", handle, runtime.GOOS)
}
