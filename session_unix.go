//go:build unix

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
	case syscall.EAGAIN, syscall.EINTR:
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
	case syscall.ECONNRESET, syscall.EPIPE, syscall.ENOTCONN, syscall.EBADF, syscall.ECONNABORTED:
		return true
	}
	return false
}
