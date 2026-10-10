//go:build !windows

package main

import "os"

func openSource(path string) (*os.File, error) {
	return os.Open(path)
}

func nativePath(path string) string {
	return path
}

func shortBaseName(path string) string {
	return ""
}
