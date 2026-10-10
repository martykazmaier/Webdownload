package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// DropFile is a DOOR32.SYS revision 1 drop file.
type DropFile struct {
	CommType   int    // 0=local, 1=serial, 2=telnet/socket
	Handle     uintptr
	Baud       int
	BBSID      string
	UserRec    int
	RealName   string
	HandleName string
	Security   int
	TimeLeft   time.Duration
	Emulation  int // 0=ASCII, 1=ANSI, ...
	Node       int
	Path       string
}

func findDoor32(explicit string) (string, error) {
	candidates := []string{}
	if explicit != "" {
		candidates = append(candidates, explicit)
	}
	candidates = append(candidates,
		"DOOR32.SYS",
		"door32.sys",
		"Door32.sys",
		"door32.SYS",
	)
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("DOOR32.SYS not found")
}

func readDoor32(path string) (*DropFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseDoor32(path, data)
}

func parseDoor32(path string, data []byte) (*DropFile, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, ln := range raw {
		lines = append(lines, strings.TrimRight(ln, " \t"))
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) < 11 {
		return nil, fmt.Errorf("%s: expected 11 lines, got %d", path, len(lines))
	}

	comm, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return nil, fmt.Errorf("%s: comm type: %w", path, err)
	}
	handle, err := strconv.ParseUint(strings.TrimSpace(lines[1]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s: socket handle: %w", path, err)
	}
	baud, _ := strconv.Atoi(strings.TrimSpace(lines[2]))
	userRec, _ := strconv.Atoi(strings.TrimSpace(lines[4]))
	sec, _ := strconv.Atoi(strings.TrimSpace(lines[7]))
	mins, _ := strconv.Atoi(strings.TrimSpace(lines[8]))
	emu, _ := strconv.Atoi(strings.TrimSpace(lines[9]))
	node, _ := strconv.Atoi(strings.TrimSpace(lines[10]))
	if mins < 0 {
		mins = 0
	}

	return &DropFile{
		CommType:   comm,
		Handle:     uintptr(handle),
		Baud:       baud,
		BBSID:      strings.TrimSpace(lines[3]),
		UserRec:    userRec,
		RealName:   strings.TrimSpace(lines[5]),
		HandleName: strings.TrimSpace(lines[6]),
		Security:   sec,
		TimeLeft:   time.Duration(mins) * time.Minute,
		Emulation:  emu,
		Node:       node,
		Path:       path,
	}, nil
}
