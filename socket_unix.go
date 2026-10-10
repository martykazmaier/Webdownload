//go:build unix

package main

import (
	"net"
	"os"

	"golang.org/x/sys/unix"
)

// inheritedConn is a Door32 socket we must not shut down; EleBBS still owns
// the live telnet session after this door returns.
type inheritedConn struct {
	net.Conn
}

func (inheritedConn) Close() error { return nil }

func connFromHandle(handle uintptr) (net.Conn, error) {
	fd := int(handle)
	if fd <= 0 {
		return nil, errInvalidHandle
	}
	_ = unix.SetNonblock(fd, false)
	_ = unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_NODELAY, 1)

	f := os.NewFile(uintptr(fd), "door32")
	if f == nil {
		return nil, errInvalidHandle
	}
	conn, err := net.FileConn(f)
	_ = f.Close()
	if err != nil {
		return nil, err
	}
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}
	return inheritedConn{Conn: conn}, nil
}
