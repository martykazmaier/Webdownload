package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	randomDirLen     = 16
	webfilesDir      = "webfiles"
	maxDownloadBytes = int64(5) * 1024 * 1024 * 1024
	minFreeBytes     = int64(20) * 1024 * 1024 * 1024
)

type CopiedFile struct {
	Source string
	Name   string
	Size   int64
}

func sanitizeUser(name string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), ".")
	if out == "" {
		return "user"
	}
	switch strings.ToUpper(out) {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4",
		"LPT1", "LPT2", "LPT3":
		return "_" + out
	}
	return out
}

func randomHexDir() (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	s := hex.EncodeToString(raw[:])
	if len(s) != randomDirLen {
		return "", fmt.Errorf("expected %d hex chars, got %d", randomDirLen, len(s))
	}
	return s, nil
}

func userWebfilesDir(webdir, user string) string {
	return filepath.Join(filepath.Clean(webdir), webfilesDir, sanitizeUser(user))
}

func makeDownloadDir(webdir, user string) (subdir, dest string, err error) {
	parent := userWebfilesDir(webdir, user)
	if err = os.MkdirAll(parent, 0o755); err != nil {
		return "", "", fmt.Errorf("create %s: %w", parent, err)
	}
	for i := 0; i < 8; i++ {
		subdir, err = randomHexDir()
		if err != nil {
			return "", "", err
		}
		dest = filepath.Join(parent, subdir)
		err = os.Mkdir(dest, 0o755)
		if err == nil {
			return subdir, dest, nil
		}
		if os.IsExist(err) {
			continue
		}
		return "", "", fmt.Errorf("create %s: %w", dest, err)
	}
	return "", "", fmt.Errorf("could not allocate a unique download directory")
}

func ctlSource(f CTLFile) string {
	if f.Path != "" {
		return f.Path
	}
	return quotedJoin(f.Dir, f.Name)
}

func timedSize(path string) (int64, error) {
	in, err := timedOpen(path)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return 0, err
	}
	if st.IsDir() {
		return 0, fmt.Errorf("is a directory")
	}
	return st.Size(), nil
}

func totalCTLSize(files []CTLFile) (int64, error) {
	var total int64
	for _, f := range files {
		src := ctlSource(f)
		n, err := timedSize(src)
		if err != nil {
			continue
		}
		total += n
		if overDownloadLimit(total) {
			return total, errDownloadTooLarge(total)
		}
	}
	return total, nil
}

func overDownloadLimit(total int64) bool {
	return total > maxDownloadBytes
}

func errDownloadTooLarge(total int64) error {
	return fmt.Errorf("total file size %s is more than 5 GB", formatSize(total))
}

func destHasFreeSpace(free int64) bool {
	return free >= minFreeBytes
}

func errDestLowSpace(free int64) error {
	return fmt.Errorf("destination drive has %s free; 20 GB is required", formatSize(free))
}

func checkDestFree(path string) error {
	free, err := diskFree(path)
	if err != nil {
		return fmt.Errorf("check free space: %w", err)
	}
	if !destHasFreeSpace(free) {
		return errDestLowSpace(free)
	}
	return nil
}

type copyProgress struct {
	Name      string
	FileNum   int
	FileCount int
	FileDone  int64
	FileSize  int64
	AllDone   int64
	AllTotal  int64
}

type countingWriter struct {
	w  io.Writer
	n  int64
	fn func(int64)
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	if n > 0 {
		c.n += int64(n)
		if c.fn != nil {
			c.fn(c.n)
		}
	}
	return n, err
}

func copyCTLFiles(files []CTLFile, dest string) (copied []CopiedFile, errs []error) {
	return copyCTLFilesProgress(files, dest, 0, nil)
}

func copyCTLFilesProgress(files []CTLFile, dest string, allTotal int64, report func(copyProgress)) (copied []CopiedFile, errs []error) {
	used := map[string]int{}
	var allDone int64
	nfiles := len(files)
	for i, f := range files {
		src := ctlSource(f)
		name := filepath.Base(src)
		if report != nil {
			report(copyProgress{
				Name:      name,
				FileNum:   i + 1,
				FileCount: nfiles,
				FileSize:  0,
				AllDone:   allDone,
				AllTotal:  allTotal,
			})
		}
		cf, err := copyOne(src, dest, used, func(fileDone, fileSize int64) {
			if report == nil {
				return
			}
			report(copyProgress{
				Name:      name,
				FileNum:   i + 1,
				FileCount: nfiles,
				FileDone:  fileDone,
				FileSize:  fileSize,
				AllDone:   allDone + fileDone,
				AllTotal:  allTotal,
			})
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", src, err))
			continue
		}
		allDone += cf.Size
		copied = append(copied, cf)
		if report != nil {
			report(copyProgress{
				Name:      cf.Name,
				FileNum:   i + 1,
				FileCount: nfiles,
				FileDone:  cf.Size,
				FileSize:  cf.Size,
				AllDone:   allDone,
				AllTotal:  allTotal,
			})
		}
	}
	return copied, errs
}

func copyOne(src, dest string, used map[string]int, report func(done, size int64)) (CopiedFile, error) {
	in, err := timedOpen(src)
	if err != nil {
		return CopiedFile{}, err
	}
	defer in.Close()

	st, err := in.Stat()
	if err != nil {
		return CopiedFile{}, err
	}
	if st.IsDir() {
		return CopiedFile{}, fmt.Errorf("is a directory")
	}

	name := uniqueName(filepath.Base(src), used)
	outPath := filepath.Join(dest, name)
	out, err := os.OpenFile(outPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return CopiedFile{}, err
	}
	cw := &countingWriter{w: out, fn: func(n int64) {
		if report != nil {
			report(n, st.Size())
		}
	}}
	n, copyErr := io.Copy(cw, in)
	closeErr := out.Close()
	if copyErr != nil {
		return CopiedFile{}, copyErr
	}
	if closeErr != nil {
		return CopiedFile{}, closeErr
	}
	return CopiedFile{Source: src, Name: name, Size: n}, nil
}

func uniqueName(base string, used map[string]int) string {
	key := strings.ToLower(base)
	if used[key] == 0 {
		used[key] = 1
		return base
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for {
		used[key]++
		name := fmt.Sprintf("%s_%d%s", stem, used[key], ext)
		lk := strings.ToLower(name)
		if used[lk] == 0 {
			used[lk] = 1
			return name
		}
	}
}

func writeIndex(dest, baseURL, user, subdir string, files []CopiedFile) error {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>SHS BBS Downloads</title>
<style>
body { background:#000; color:#00ff66; font-family: "Lucida Console", "Courier New", monospace; margin: 2rem; }
h1 { color:#66ffff; font-weight: normal; }
a { color:#66ffff; }
ul { list-style: square; }
.meta { color:#888; }
</style>
</head>
<body>
`)
	fmt.Fprintf(&b, "<h1>Downloads for %s</h1>\n", html.EscapeString(user))
	fmt.Fprintf(&b, `<p class="meta">%s/%s/%s/%s/</p>`+"\n", html.EscapeString(strings.TrimRight(baseURL, "/")), webfilesDir, html.EscapeString(user), html.EscapeString(subdir))
	if len(files) == 0 {
		b.WriteString("<p>No files were copied.</p>\n")
	} else {
		b.WriteString("<ul>\n")
		for _, f := range files {
			fmt.Fprintf(&b, `<li><a href="%s" download="%s">%s</a> <span class="meta">(%s)</span></li>`+"\n",
				html.EscapeString(url.PathEscape(f.Name)), html.EscapeString(f.Name), html.EscapeString(f.Name), formatSize(f.Size))
		}
		b.WriteString("</ul>\n")
	}
	b.WriteString("</body>\n</html>\n")
	return os.WriteFile(filepath.Join(dest, "index.html"), []byte(b.String()), 0o644)
}

func normalizeBaseURL(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "/")
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	return s
}

func publicURL(base, user, subdir string) string {
	return fmt.Sprintf("%s/%s/%s/%s/", strings.TrimRight(base, "/"), webfilesDir, user, subdir)
}

func formatSize(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d bytes", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	default:
		return fmt.Sprintf("%.1f GB", float64(n)/(1024*1024*1024))
	}
}
