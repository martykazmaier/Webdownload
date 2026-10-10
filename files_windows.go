//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func openSource(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(nativePath(path))
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(
		p,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_SEQUENTIAL_SCAN,
		0,
	)
	if err != nil {
		return os.Open(nativePath(path))
	}
	return os.NewFile(uintptr(h), path), nil
}

func nativePath(path string) string {
	path = filepath.Clean(path)
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, `\\?\`) || strings.HasPrefix(path, `\\.\`) {
		return path
	}
	if strings.HasPrefix(path, `\\`) {
		return `\\?\UNC\` + strings.TrimPrefix(path, `\\`)
	}
	if len(path) >= 2 && path[1] == ':' {
		return `\\?\` + path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	if strings.HasPrefix(abs, `\\`) {
		return `\\?\UNC\` + strings.TrimPrefix(abs, `\\`)
	}
	return `\\?\` + abs
}

func stripExtendedPrefix(p string) string {
	switch {
	case strings.HasPrefix(p, `\\?\UNC\`):
		return `\\` + strings.TrimPrefix(p, `\\?\UNC\`)
	case strings.HasPrefix(p, `\\?\`):
		return strings.TrimPrefix(p, `\\?\`)
	default:
		return p
	}
}

func shortPathName(path string) string {
	if path == "" {
		return ""
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	buf := make([]uint16, 32768)
	n, _ := windows.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if n == 0 || n > uint32(len(buf)) {
		return ""
	}
	return windows.UTF16ToString(buf)
}

func shortBaseName(path string) string {
	path = filepath.Clean(path)
	for _, candidate := range []string{path, nativePath(path)} {
		s := stripExtendedPrefix(shortPathName(candidate))
		if s == "" {
			continue
		}
		base := filepath.Base(s)
		if base == "" || base == "." || strings.ContainsAny(base, `/\`) {
			continue
		}
		return base
	}
	return ""
}

func diskFree(path string) (int64, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	p, err := windows.UTF16PtrFromString(abs)
	if err != nil {
		return 0, err
	}
	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &free, &total, &totalFree); err != nil {
		return 0, err
	}
	if free > 1<<63-1 {
		return 1<<63 - 1, nil
	}
	return int64(free), nil
}
