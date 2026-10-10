//go:build !windows && !unix

package main

import "io"

func isRetryableWait(err error) bool {
	if err == nil || err == io.EOF {
		return false
	}
	if ne, ok := err.(interface{ Timeout() bool }); ok && ne.Timeout() {
		return false
	}
	return false
}

func isDisconnect(err error) bool {
	return err == io.EOF
}
