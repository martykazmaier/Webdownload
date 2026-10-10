package main

import (
	"bytes"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWaitEscapeAcceptsCtrlX(t *testing.T) {
	if err := waitEscapeRaw(strings.NewReader("\x18")); err != nil {
		t.Fatal(err)
	}
	if err := waitEscapeRaw(strings.NewReader("\x1b")); err != nil {
		t.Fatal(err)
	}
}

func TestSpacedFilenameIsSinglePath(t *testing.T) {
	f, err := resolveCTLLine(`C:\FILES\GAME ROM.ZIP`)
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != "GAME ROM.ZIP" {
		t.Fatalf("name %+v", f)
	}
	want := filepath.Clean(`C:\FILES\GAME ROM.ZIP`)
	if f.Path != want {
		t.Fatalf("path %q want %q", f.Path, want)
	}
	f, ok := splitDriveDirAndName(`C:\FILES GAME ROM.ZIP`)
	if !ok || f.Dir != `C:\FILES` || f.Name != "GAME ROM.ZIP" {
		t.Fatalf("%+v %v", f, ok)
	}
	u, err := resolveCTLLine(`/bbs/files/GAME ROM.ZIP`)
	if err != nil {
		t.Fatal(err)
	}
	if u.Name != "GAME ROM.ZIP" {
		t.Fatalf("unix name %+v", u)
	}
}

func TestAppendDSZLogKeepsLongFileName(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "GAME ROM.ZIP")
	if err := os.WriteFile(src, []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(dir, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	t.Setenv("DSZLOG", "")
	t.Setenv("dszlog", "")
	if err := appendDSZLog([]CopiedFile{{Name: "GAME ROM.ZIP", Source: src, Size: 3}}, 38400, "", ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("dszlog")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "W 3 38400 bps 9999 cps 0 errors 0 1024 GAME ROM.ZIP") {
		t.Fatalf("missing long name in dszlog: %q", data)
	}
}

func TestParseDoor32(t *testing.T) {
	raw := "2\r\n" +
		"1234\r\n" +
		"38400\r\n" +
		"EleBBS 0.76\r\n" +
		"1\r\n" +
		"Martin Designer\r\n" +
		"Martin\r\n" +
		"255\r\n" +
		"58\r\n" +
		"1\r\n" +
		"3\r\n"
	d, err := parseDoor32("DOOR32.SYS", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if d.CommType != 2 || d.Handle != 1234 {
		t.Fatalf("socket fields: %+v", d)
	}
	if d.HandleName != "Martin" || d.RealName != "Martin Designer" {
		t.Fatalf("names: %+v", d)
	}
	if d.TimeLeft != 58*time.Minute {
		t.Fatalf("time left: %s", d.TimeLeft)
	}
	if d.Emulation != 1 || d.Node != 3 {
		t.Fatalf("emu/node: %+v", d)
	}
}

func TestParseDSZFullPathAndDirName(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "GAME.ZIP")
	bdir := filepath.Join(dir, "utils")
	if err := os.Mkdir(bdir, 0o755); err != nil {
		t.Fatal(err)
	}
	b := filepath.Join(bdir, "PKUNZIP.EXE")
	if err := os.WriteFile(a, []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctl := a + "\r\n" + `"` + bdir + `" PKUNZIP.EXE` + "\r\n; comment\r\n\r\n"
	files, err := parseDSZData("DSZ.CTL", ctl)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files: %+v", len(files), files)
	}
	if files[0].Path != filepath.Clean(a) {
		t.Fatalf("file 0: %s", files[0].Path)
	}
	if files[1].Path != filepath.Clean(b) {
		t.Fatalf("file 1: %s", files[1].Path)
	}
}

func TestFindDSZBesideDropFile(t *testing.T) {
	dir := t.TempDir()
	drop := filepath.Join(dir, "DOOR32.SYS")
	ctl := filepath.Join(dir, "DSZ.CTL")
	if err := os.WriteFile(drop, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ctl, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := findDSZ(drop)
	if err != nil {
		t.Fatal(err)
	}
	if got != ctl {
		t.Fatalf("got %q want %q", got, ctl)
	}
}

func TestParseAndCopyFilenameWithSpaces(t *testing.T) {
	dir := t.TempDir()
	spaced := filepath.Join(dir, "GAME ROM.ZIP")
	if err := os.WriteFile(spaced, []byte("zipdata"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctl := spaced + "\r\n"
	files, err := parseDSZData("DSZ.CTL", ctl)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name != "GAME ROM.ZIP" || files[0].Path != filepath.Clean(spaced) {
		t.Fatalf("parsed %+v", files)
	}

	quoted := `"` + dir + `" "GAME ROM.ZIP"` + "\r\n"
	files, err = parseDSZData("DSZ.CTL", quoted)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != filepath.Clean(spaced) {
		t.Fatalf("quoted %+v", files)
	}

	web := t.TempDir()
	_, dest, err := makeDownloadDir(web, "Martin")
	if err != nil {
		t.Fatal(err)
	}
	copied, errs := copyCTLFiles(files, dest)
	if len(errs) != 0 || len(copied) != 1 {
		t.Fatalf("copy: %+v %+v", copied, errs)
	}
	if copied[0].Name != "GAME ROM.ZIP" {
		t.Fatalf("copied name %q", copied[0].Name)
	}
	if _, err := os.Stat(filepath.Join(dest, "GAME ROM.ZIP")); err != nil {
		t.Fatal(err)
	}
}

func TestDSZLogKeepsLongFileName(t *testing.T) {
	names := dszLogFileNames(CopiedFile{Name: "GAME ROM.ZIP", Source: `C:\FILES\GAME ROM.ZIP`})
	if len(names) == 0 || names[0] != "GAME ROM.ZIP" {
		t.Fatalf("long name first: %q", names)
	}
	got := formatDSZLogLine("GAME ROM.ZIP", 100, 38400)
	want := "W 100 38400 bps 9999 cps 0 errors 0 1024 GAME ROM.ZIP\r\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatDSZLogLine(t *testing.T) {
	got := formatDSZLogLine("file.zip", 46532, 38400)
	want := "W 46532 38400 bps 9999 cps 0 errors 0 1024 FILE.ZIP\r\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	lines := formatDSZLogLines("file.zip", 46532, 38400)
	if len(lines) != 3 || !strings.HasPrefix(lines[1], "Z ") || !strings.HasPrefix(lines[2], "z ") {
		t.Fatalf("lines: %q", lines)
	}
	if strings.Contains(strings.Join(lines, ""), " -1") {
		t.Fatal("trailing -1 makes EleBBS treat the serial as the filename")
	}
}

func TestAppendDSZLogUsesEnvFilenameInCwd(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	full := filepath.Join(dir, "NODE", "DSZ.LOG")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DSZLOG", full)
	if err := appendDSZLog([]CopiedFile{{Name: "FILE.ZIP", Size: 100}}, 19200, "", ""); err != nil {
		t.Fatal(err)
	}
	want := `W 100 19200 bps 9999 cps 0 errors 0 1024 FILE.ZIP`
	for _, p := range []string{full, filepath.Join(dir, "dszlog"), filepath.Join(dir, "DSZ.LOG")} {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if !strings.Contains(string(data), want) {
			t.Fatalf("%s log: %q", p, data)
		}
		if !strings.Contains(string(data), "Z 100 19200 bps 9999 cps 0 errors 0 1024 FILE.ZIP") {
			t.Fatalf("%s missing Z line: %q", p, data)
		}
	}
}

func TestAppendDSZLogBesideCTLAndOpusLog(t *testing.T) {
	dir := t.TempDir()
	ctlDir := filepath.Join(dir, "node")
	if err := os.MkdirAll(ctlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	opusLog := filepath.Join(dir, "xfer", "PROTOCOL.LOG")
	ctl := filepath.Join(ctlDir, "DSZ.CTL")
	drop := filepath.Join(ctlDir, "DOOR32.SYS")
	if err := os.WriteFile(ctl, []byte("Log "+opusLog+"\r\nC:\\FILES\\FILE.ZIP\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(drop, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(dir, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	t.Setenv("DSZLOG", "")
	if err := appendDSZLog([]CopiedFile{{Name: "FILE.ZIP", Size: 50}}, 38400, ctl, drop); err != nil {
		t.Fatal(err)
	}
	want := `W 50 38400 bps 9999 cps 0 errors 0 1024 FILE.ZIP`
	for _, p := range []string{
		filepath.Join(work, "dszlog"),
		filepath.Join(work, "DSZ.LOG"),
		filepath.Join(ctlDir, "dszlog"),
		filepath.Join(ctlDir, "DSZ.LOG"),
		opusLog,
	} {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if !strings.Contains(string(data), want) {
			t.Fatalf("%s log: %q", p, data)
		}
	}
}

func eleGetFileName(start int, orig string, nextWord int) string {
	if start < 1 {
		start = 1
	}
	if start > len(orig) {
		return ""
	}
	temp := orig[start-1:]
	wordNum := 0
	for strings.Contains(temp, " ") && wordNum != nextWord {
		i := strings.IndexByte(temp, ' ')
		temp = strings.TrimLeft(temp[i+1:], " ")
		wordNum++
	}
	if temp == "" {
		return ""
	}
	end := strings.IndexByte(temp, ' ')
	if end < 0 {
		return temp
	}
	return temp[:end]
}

func TestEleBBSGetFileNameWord10And11(t *testing.T) {
	line := strings.TrimRight(formatDSZLogLine("FILE.ZIP", 100, 38400), "\r\n")
	if got := eleGetFileName(1, line, 10); got != "FILE.ZIP" {
		t.Fatalf("word 10: %q", got)
	}
	if got := eleGetFileName(1, line, 11); got != "FILE.ZIP" {
		t.Fatalf("word 11: %q", got)
	}
	broken := line + " -1"
	if got := eleGetFileName(1, broken, 11); got == "FILE.ZIP" {
		t.Fatalf("expected -1 to steal word 11, got %q", got)
	}
}

func TestAppendDSZLogWritesBareDszlogName(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	t.Setenv("DSZLOG", "")
	t.Setenv("dszlog", "")
	if err := appendDSZLog([]CopiedFile{{Name: "GAME.ZIP", Size: 12}}, 38400, "", ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("dszlog")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "W 12 38400 bps 9999 cps 0 errors 0 1024 GAME.ZIP") {
		t.Fatalf("dszlog: %q", data)
	}
}

func TestDestFreeSpaceRequired(t *testing.T) {
	if !destHasFreeSpace(minFreeBytes) {
		t.Fatal("exactly 20 GB free should be allowed")
	}
	if destHasFreeSpace(minFreeBytes - 1) {
		t.Fatal("less than 20 GB free should error")
	}
	err := errDestLowSpace(1024)
	if err == nil || !strings.Contains(err.Error(), "20 GB is required") {
		t.Fatalf("got %v", err)
	}
}

func TestCheckDestFreeOnTempDir(t *testing.T) {
	dir := t.TempDir()
	if err := checkDestFree(dir); err != nil {
		t.Skipf("temp drive does not have 20 GB free: %v", err)
	}
}

func TestOverDownloadLimit(t *testing.T) {
	if overDownloadLimit(maxDownloadBytes) {
		t.Fatal("exactly 5 GB should be allowed")
	}
	if !overDownloadLimit(maxDownloadBytes + 1) {
		t.Fatal("more than 5 GB should error")
	}
	if overDownloadLimit(0) || overDownloadLimit(1024) {
		t.Fatal("small totals should be allowed")
	}
}

func TestTotalCTLSizeUnderLimit(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "A.ZIP")
	b := filepath.Join(dir, "B.ZIP")
	if err := os.WriteFile(a, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("67890"), 0o644); err != nil {
		t.Fatal(err)
	}
	total, err := totalCTLSize([]CTLFile{{Path: a}, {Path: b}})
	if err != nil {
		t.Fatal(err)
	}
	if total != 10 {
		t.Fatalf("total %d", total)
	}
}

func TestErrDownloadTooLargeMessage(t *testing.T) {
	err := errDownloadTooLarge(maxDownloadBytes + 1)
	if err == nil || !strings.Contains(err.Error(), "more than 5 GB") {
		t.Fatalf("got %v", err)
	}
}

func TestCTLPathArg(t *testing.T) {
	list := parseCtlArg("@C:\\BBS\\DSZ.CTL")
	if !list.List || list.Path != `C:\BBS\DSZ.CTL` {
		t.Fatalf("list: %+v", list)
	}
	one := parseCtlArg(`C:\FILES\GAME.ZIP`)
	if one.List || one.Path != `C:\FILES\GAME.ZIP` {
		t.Fatalf("file: %+v", one)
	}
	bare := parseCtlArg("@")
	if !bare.List || bare.Path != "" {
		t.Fatalf("@ alone: %+v", bare)
	}
	if got := ctlArgFromArgs([]string{"https://shsbbs.net", `C:\WWW`, "Martin"}); !got.List || got.Path != "" {
		t.Fatalf("omitted: %+v", got)
	}
	if got := ctlArgFromArgs([]string{"https://shsbbs.net", `C:\WWW`, "Martin", `@C:\NODE\FILES.CTL`}); !got.List || got.Path != `C:\NODE\FILES.CTL` {
		t.Fatalf("list arg: %+v", got)
	}
	if got := ctlArgFromArgs([]string{"https://shsbbs.net", `C:\WWW`, "Martin", `C:\BBS\My`, `Files\GAME.ZIP`}); got.List || got.Path != `C:\BBS\My Files\GAME.ZIP` {
		t.Fatalf("spaced file: %+v", got)
	}
}

func TestLoadTaggedFilesSingle(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "GAME.ZIP")
	if err := os.WriteFile(src, []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, listPath, err := loadTaggedFiles(ctlArg{List: false, Path: src}, "")
	if err != nil {
		t.Fatal(err)
	}
	if listPath != "" || len(files) != 1 || files[0].Path != filepath.Clean(src) {
		t.Fatalf("files %+v list %q", files, listPath)
	}
}

func TestPublicURL(t *testing.T) {
	got := publicURL("https://shsbbs.net", "Martin", "0123456789abcdef")
	want := "https://shsbbs.net/webfiles/Martin/0123456789abcdef/"
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	if got := normalizeBaseURL("https://example.com/"); got != "https://example.com" {
		t.Fatalf("normalize slash: %q", got)
	}
	if got := normalizeBaseURL("example.com"); got != "https://example.com" {
		t.Fatalf("normalize host: %q", got)
	}
}

func TestSanitizeUser(t *testing.T) {
	if got := sanitizeUser("Mar tin!"); got != "Mar_tin_" {
		t.Fatalf("got %q", got)
	}
}

func TestCopyAndIndex(t *testing.T) {
	srcDir := t.TempDir()
	web := t.TempDir()
	src := filepath.Join(srcDir, "FILE.ZIP")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	subdir, dest, err := makeDownloadDir(web, "Martin")
	if err != nil {
		t.Fatal(err)
	}
	if len(subdir) != 16 {
		t.Fatalf("subdir len %d: %s", len(subdir), subdir)
	}
	copied, errs := copyCTLFiles([]CTLFile{{Path: src, Line: 1}}, dest)
	if len(errs) != 0 || len(copied) != 1 {
		t.Fatalf("copy: %+v %+v", copied, errs)
	}
	if err := writeIndex(dest, "https://shsbbs.net", "Martin", subdir, copied); err != nil {
		t.Fatal(err)
	}
	idx, err := os.ReadFile(filepath.Join(dest, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(idx)
	if !strings.Contains(html, "FILE.ZIP") || !strings.Contains(html, publicURL("https://shsbbs.net", "Martin", subdir)) {
		t.Fatalf("index missing content:\n%s", html)
	}
	if _, err := os.Stat(filepath.Join(web, "webfiles", "Martin", subdir, "FILE.ZIP")); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupUserDirExpiresOld(t *testing.T) {
	web := t.TempDir()
	user := "Martin"
	now := time.Now()

	oldDir := filepath.Join(web, webfilesDir, user, "oldoldoldoldold1")
	newDir := filepath.Join(web, webfilesDir, user, "newnewnewnewnew2")
	keepUser := filepath.Join(web, webfilesDir, user)
	outside := filepath.Join(web, user, "stale_drop")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "gone.zip"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := now.Add(-25 * time.Hour)
	if err := os.Chtimes(oldDir, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(outside, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	removed, errs := cleanupUserDir(web, user, now, 24*time.Hour)
	if len(errs) != 0 {
		t.Fatalf("cleanup errs: %v", errs)
	}
	if len(removed) != 1 {
		t.Fatalf("removed %d: %+v", len(removed), removed)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("old download dir still present: %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(newDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keepUser); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupAllUsers(t *testing.T) {
	web := t.TempDir()
	now := time.Now()
	oldA := filepath.Join(web, webfilesDir, "Alice", "aaaaaaaaaaaaaaaa")
	oldB := filepath.Join(web, webfilesDir, "Bob", "bbbbbbbbbbbbbbbb")
	fresh := filepath.Join(web, webfilesDir, "Alice", "cccccccccccccccc")
	if err := os.MkdirAll(oldA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(oldB, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fresh, 0o755); err != nil {
		t.Fatal(err)
	}
	oldTime := now.Add(-25 * time.Hour)
	if err := os.Chtimes(oldA, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(oldB, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	removed, errs := cleanupAllUsers(web, now, 24*time.Hour)
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(removed) != 2 {
		t.Fatalf("removed %d: %+v", len(removed), removed)
	}
	if _, err := os.Stat(oldA); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldB); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatal(err)
	}
}

func TestWaitEscapeIgnoresLaunchCR(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		_, _ = client.Write([]byte("\r\n"))
		time.Sleep(80 * time.Millisecond)
		_, _ = client.Write([]byte{keyEscape})
		_ = client.Close()
	}()

	start := time.Now()
	if err := waitEscape(server, 3*time.Second); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if time.Since(start) < 50*time.Millisecond {
		t.Fatal("returned on leftover CR instead of waiting for Escape")
	}
}

func TestThermometerFill(t *testing.T) {
	if got := thermometerFill(10, 0, 100); got != "----------" {
		t.Fatalf("empty: %q", got)
	}
	if got := thermometerFill(10, 100, 100); got != "##########" {
		t.Fatalf("full: %q", got)
	}
	got := thermometerFill(10, 50, 100)
	if !strings.Contains(got, "#") || !strings.Contains(got, "-") || len(got) != 10 {
		t.Fatalf("half: %q", got)
	}
	if percent(0, 100) != 0 || percent(50, 100) != 50 || percent(100, 100) != 100 {
		t.Fatalf("percent %d %d %d", percent(0, 100), percent(50, 100), percent(100, 100))
	}
}

func TestRenderProgressThermometer(t *testing.T) {
	s := string(renderProgress(copyProgress{
		Name: "GAME.ZIP", FileNum: 1, FileCount: 2,
		FileDone: 50, FileSize: 100, AllDone: 50, AllTotal: 100,
	}))
	if !strings.Contains(s, "GAME.ZIP") || !strings.Contains(s, "50%") || !strings.Contains(s, "[") {
		t.Fatalf("screen: %q", s)
	}
	if !strings.Contains(s, "#") || !strings.Contains(s, "Copying file 1 of 2") {
		t.Fatalf("bar: %q", s)
	}
}

func TestCopyReportsProgress(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "BIG.ZIP")
	payload := bytes.Repeat([]byte("x"), 128*1024)
	if err := os.WriteFile(src, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	var reports []copyProgress
	copied, errs := copyCTLFilesProgress([]CTLFile{{Path: src, Line: 1}}, dest, int64(len(payload)), func(p copyProgress) {
		reports = append(reports, p)
	})
	if len(errs) != 0 || len(copied) != 1 {
		t.Fatalf("copy: %+v %+v", copied, errs)
	}
	if len(reports) < 2 {
		t.Fatalf("expected progress updates, got %d", len(reports))
	}
	last := reports[len(reports)-1]
	if last.AllDone != int64(len(payload)) || last.AllTotal != int64(len(payload)) {
		t.Fatalf("last %+v", last)
	}
}
