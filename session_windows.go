//go:build windows

package main

import (
	"io"
	"syscall"
)

func isRetryableWait(err error) bool {
	if err == nil || err == io.EOF {
		return false
	}
	if ne, ok := err.(interface{ Timeout() bool }); ok && ne.Timeout() {
		return false
	}
	errno, ok := err.(syscall.Errno)
	if !ok {
		return false
	}
	switch errno {
	case 10035, 10004, 10036: // WSAEWOULDBLOCK, WSAEINTR, WSAEINPROGRESS
		return true
	}
	return false
}

func isDisconnect(err error) bool {
	if err == nil {
		return false
	}
	if err == io.EOF {
		return true
	}
	errno, ok := err.(syscall.Errno)
	if !ok {
		return false
	}
	switch errno {
	case 10054, 10053, 10057, 10038: // RESET, ABORTED, NOTCONN, NOTSOCK
		return true
	}
	return false
}
