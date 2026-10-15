package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RA 2.50 / EleBBS packed CONFIGrecord: UserTimeOut is a Word of idle
// seconds at this offset. Same layout used by the other EleBBS doors.
const userTimeOutOff = 1479

func raDir() string {
	for _, k := range []string{"RA", "ELEBBS"} {
		if d := strings.TrimSpace(os.Getenv(k)); d != "" {
			return d
		}
	}
	return ""
}

func configRAPath() string {
	dir := raDir()
	if dir == "" {
		return ""
	}
	for _, name := range []string{"CONFIG.RA", "config.ra", "Config.ra"} {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return filepath.Join(dir, "CONFIG.RA")
}

func idleFromConfigRA() time.Duration {
	p := configRAPath()
	if p == "" {
		return 0
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return 0
	}
	return idleFromConfigRAData(data)
}

func idleFromConfigRAData(data []byte) time.Duration {
	if len(data) < userTimeOutOff+2 {
		return 0
	}
	secs := binary.LittleEndian.Uint16(data[userTimeOutOff:])
	if secs == 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}
