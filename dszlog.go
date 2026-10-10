package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func dszLogName() string {
	v := strings.TrimSpace(os.Getenv("DSZLOG"))
	if v == "" {
		v = strings.TrimSpace(os.Getenv("dszlog"))
	}
	if v != "" {
		return v
	}
	// EleBBS protocol "Log file" is a path, not an env var. Sysops often
	// set it to "dszlog" (no extension). That is not the same file as DSZ.LOG.
	return "dszlog"
}

func opusLogPath(ctlPath string) string {
	if ctlPath == "" {
		return ""
	}
	data, err := os.ReadFile(ctlPath)
	if err != nil {
		return ""
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	for _, raw := range strings.Split(text, "\n") {
		ln := strings.TrimSpace(raw)
		if ln == "" {
			continue
		}
		fields := strings.Fields(ln)
		if len(fields) >= 2 && strings.EqualFold(fields[0], "Log") {
			// Remainder of the line is the protocol log path (may contain spaces).
			idx := strings.IndexFunc(ln, func(r rune) bool { return r == ' ' || r == '\t' })
			if idx < 0 {
				continue
			}
			return strings.TrimSpace(ln[idx+1:])
		}
	}
	return ""
}

func dszLogTargets(ctlPath, dropPath string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		abs, err := filepath.Abs(p)
		key := strings.ToLower(filepath.Clean(p))
		if err == nil {
			key = strings.ToLower(filepath.Clean(abs))
		}
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, p)
	}
	addInDir := func(dir, name string) {
		if dir == "" || name == "" {
			return
		}
		add(filepath.Join(dir, filepath.Base(name)))
	}

	envPath := dszLogName()
	base := filepath.Base(envPath)
	add(envPath)
	add(filepath.Join(".", base))
	add("dszlog")
	add("DSZ.LOG")
	if p := opusLogPath(ctlPath); p != "" {
		add(p)
		add(filepath.Join(".", filepath.Base(p)))
	}

	var dirs []string
	if ctlPath != "" {
		dirs = append(dirs, filepath.Dir(ctlPath))
	}
	if dropPath != "" {
		dirs = append(dirs, filepath.Dir(dropPath))
	}
	for _, dir := range dirs {
		if dir == "" || dir == "." {
			continue
		}
		addInDir(dir, base)
		addInDir(dir, "dszlog")
		addInDir(dir, "DSZ.LOG")
	}
	return out
}

func dszFileName(name string) string {
	base := strings.ToUpper(filepath.Base(name))
	base = strings.ReplaceAll(base, `"`, "")
	return base
}

func dszLogFileNames(cf CopiedFile) []string {
	longName := strings.TrimSpace(cf.Name)
	if longName == "" {
		longName = filepath.Base(cf.Source)
	}
	longName = strings.TrimSpace(longName)
	if longName == "" {
		return nil
	}
	out := []string{longName}
	seen := map[string]bool{strings.ToLower(longName): true}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, name)
	}
	if cf.Source != "" {
		add(shortBaseName(cf.Source))
	}
	return out
}

func formatDSZLogLine(name string, size int64, baud int) string {
	return formatDSZLogLines(name, size, baud)[0]
}

func formatDSZLogLines(name string, size int64, baud int) []string {
	if baud <= 0 {
		baud = 38400
	}
	base := dszFileName(name)
	// EleBBS GetFileName skips words until XFerNameWordNum, then takes the
	// next token. RA configs use 10 or 11. A trailing "-1" (GSZ serial)
	// becomes the filename when the word number is 11, so omit it.
	// Write W (requested) and Z/z (typical cloned ZMODEM keyword).
	line := func(code string) string {
		return fmt.Sprintf("%s %d %d bps 9999 cps 0 errors 0 1024 %s\r\n", code, size, baud, base)
	}
	return []string{line("W"), line("Z"), line("z")}
}

func appendDSZLog(files []CopiedFile, baud int, ctlPath, dropPath string) error {
	if len(files) == 0 {
		return nil
	}
	var last error
	nlines := 0
	for _, cf := range files {
		nlines += len(dszLogFileNames(cf)) * 3
	}
	for _, path := range dszLogTargets(ctlPath, dropPath) {
		if err := writeDSZLogFile(path, files, baud); err != nil {
			last = err
			fmt.Fprintf(os.Stderr, "webdownload: dszlog %s: %v\n", path, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "webdownload: wrote %d DSZ log line(s) to %s\n", nlines, path)
	}
	return last
}

func writeDSZLogFile(path string, files []CopiedFile, baud int) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(path, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, cf := range files {
		for _, name := range dszLogFileNames(cf) {
			for _, line := range formatDSZLogLines(name, cf.Size, baud) {
				if _, err := f.WriteString(line); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
