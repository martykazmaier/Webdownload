package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// CTLFile is one filespec from a DSZ-style control file.
type CTLFile struct {
	Dir  string
	Name string
	Path string
	Line int
}

func unquotePath(p string) string {
	p = strings.TrimSpace(p)
	if q, ok := unquote(p); ok {
		return strings.TrimSpace(q)
	}
	return strings.Trim(p, `"`)
}

// quotedJoin joins a directory and filename into one filesystem path.
func quotedJoin(dir, name string) string {
	dir = unquotePath(dir)
	name = unquotePath(name)
	if dir == "" {
		return name
	}
	if name == "" {
		return dir
	}
	return filepath.Join(dir, name)
}

func findDSZ(dropPath string) (string, error) {
	dir := "."
	if dropPath != "" {
		dir = filepath.Dir(dropPath)
		if dir == "" {
			dir = "."
		}
	}
	for _, name := range []string{"DSZ.CTL", "dsz.ctl", "Dsz.ctl"} {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	if dropPath != "" {
		return "", fmt.Errorf("control file not found in %s (looked for DSZ.CTL beside the drop file)", dir)
	}
	return "", fmt.Errorf("control file not found in the current directory (looked for DSZ.CTL)")
}

// ctlArg is the optional 4th argument: a list file (@...) or one tagged file.
type ctlArg struct {
	List bool
	Path string
}

func parseCtlArg(raw string) ctlArg {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ctlArg{List: true}
	}
	if q, ok := unquote(raw); ok {
		raw = strings.TrimSpace(q)
	}
	if strings.HasPrefix(raw, "@") {
		path := strings.TrimSpace(strings.TrimPrefix(raw, "@"))
		return ctlArg{List: true, Path: unquotePath(path)}
	}
	return ctlArg{List: false, Path: unquotePath(raw)}
}

func ctlArgFromArgs(args []string) ctlArg {
	if len(args) < 4 {
		return ctlArg{List: true}
	}
	return parseCtlArg(strings.Join(args[3:], " "))
}

func loadTaggedFiles(spec ctlArg, dropPath string) (files []CTLFile, listPath string, err error) {
	if spec.List {
		listPath = spec.Path
		if listPath == "" {
			listPath, err = findDSZ(dropPath)
			if err != nil {
				return nil, "", err
			}
		}
		files, err = parseDSZ(listPath)
		return files, listPath, err
	}
	if spec.Path == "" {
		return nil, "", fmt.Errorf("no file specified")
	}
	f, err := resolveCTLLine(spec.Path)
	if err != nil {
		return nil, "", err
	}
	f.Line = 1
	return []CTLFile{f}, "", nil
}

func parseDSZ(path string) ([]CTLFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read control file %s: %w", path, err)
	}
	return parseDSZData(path, string(data))
}

func parseDSZData(origin, text string) ([]CTLFile, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var out []CTLFile
	for i, raw := range strings.Split(text, "\n") {
		lineNo := i + 1
		ln := strings.TrimSpace(raw)
		if ln == "" || ln[0] == ';' || ln[0] == '#' || isCTLDirective(ln) {
			continue
		}
		p, err := resolveCTLLine(ln)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", origin, lineNo, err)
		}
		if p.Path == "" {
			continue
		}
		p.Line = lineNo
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no files listed", origin)
	}
	return out, nil
}

func resolveCTLLine(ln string) (CTLFile, error) {
	ln = strings.TrimSpace(ln)
	if ln == "" {
		return CTLFile{}, fmt.Errorf("empty path")
	}

	if dir, name, ok := twoQuotedPaths(ln); ok {
		p := filepath.Join(dir, name)
		return CTLFile{Dir: dir, Name: name, Path: filepath.Clean(p)}, nil
	}

	p := unquotePath(ln)
	// C:\DIR\FILE NAME.ZIP — space is in the filename, one path. Do not Stat
	// C:\DIR\FILE as a prefix; that hangs on Windows and leaves the terminal
	// in Zmodem receive mode.
	if isFullFilePath(p) {
		return ctlFromPath(p), nil
	}
	if f, ok := splitDriveDirAndName(p); ok {
		return f, nil
	}
	if p != "" {
		return ctlFromPath(p), nil
	}
	return CTLFile{}, fmt.Errorf("empty path")
}

func isFullFilePath(p string) bool {
	n := strings.ReplaceAll(p, `\`, `/`)
	if strings.HasPrefix(p, "/") || strings.HasPrefix(n, "/") {
		return true
	}
	win := strings.ReplaceAll(p, `/`, `\`)
	if strings.HasPrefix(win, `\\`) {
		return true
	}
	if len(win) >= 3 && win[1] == ':' && win[2] == '\\' {
		return strings.Count(win[3:], `\`) >= 1
	}
	return false
}

func splitDriveDirAndName(p string) (CTLFile, bool) {
	p = strings.ReplaceAll(unquotePath(p), `/`, `\`)
	if len(p) < 4 || p[1] != ':' || p[2] != '\\' {
		return CTLFile{}, false
	}
	rest := p[3:]
	if strings.Contains(rest, `\`) {
		return CTLFile{}, false
	}
	fields := strings.Fields(rest)
	if len(fields) < 2 {
		return CTLFile{}, false
	}
	dir := p[:3] + fields[0]
	name := strings.Join(fields[1:], " ")
	return CTLFile{Dir: dir, Name: name, Path: filepath.Join(dir, name)}, true
}

func twoQuotedPaths(ln string) (dir, name string, ok bool) {
	if !strings.ContainsAny(ln, `"'`) {
		return "", "", false
	}
	fields := splitPathFields(ln)
	if len(fields) != 2 {
		return "", "", false
	}
	dir = unquotePath(fields[0])
	name = unquotePath(fields[1])
	return dir, name, dir != "" && name != ""
}

func isCTLDirective(ln string) bool {
	fields := strings.Fields(ln)
	if len(fields) == 0 {
		return false
	}
	switch strings.ToLower(fields[0]) {
	case "port", "baud", "log", "time", "com", "modem", "est", "address":
		return true
	}
	return false
}

func ctlFromPath(p string) CTLFile {
	p = filepath.Clean(p)
	return CTLFile{Dir: filepath.Dir(p), Name: filepath.Base(p), Path: p}
}

func splitPathFields(s string) []string {
	var fields []string
	var cur strings.Builder
	inQuote := rune(0)
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		fields = append(fields, cur.String())
		cur.Reset()
	}
	for _, r := range s {
		switch {
		case inQuote != 0:
			if r == inQuote {
				inQuote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			inQuote = r
		case unicode.IsSpace(r):
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return fields
}

func unquote(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1], true
		}
	}
	return s, false
}
