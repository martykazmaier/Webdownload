// Copyright (C) 2026 Martin Kazmaier.
// Distributed under the Q Public License version 1.0. See LICENSE.

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(argv []string) int {
	fs := flag.NewFlagSet("webdownload", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	door32Path := fs.String("door32", "", "path to DOOR32.SYS (default: look in the current directory)")
	local := fs.Bool("local", false, "use stdin/stdout instead of the Door32 socket")
	useWS := fs.Bool("ws", false, "frame ANSI as gorilla websocket messages")
	cleanup := fs.Bool("cleanup", false, "expire download dirs under webdir\\webfiles older than 24 hours, then exit")
	if err := fs.Parse(argv); err != nil {
		return 2
	}
	args := fs.Args()
	if *cleanup {
		if len(args) < 1 || len(args) > 2 {
			fmt.Fprintf(os.Stderr, "usage: %s -cleanup <webdir> [username]\n", progName())
			fmt.Fprintf(os.Stderr, "  omit username to clean every account under webdir/webfiles\n")
			return 2
		}
		user := ""
		if len(args) == 2 {
			user = strings.TrimSpace(args[1])
		}
		return runCleanup(strings.TrimSpace(args[0]), user)
	}
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: %s [options] <weburl> <webdir> <username> [ctlfile]\n", progName())
		fmt.Fprintf(os.Stderr, "       %s -cleanup <webdir> [username]\n", progName())
		fmt.Fprintf(os.Stderr, "  Windows: webdownload.exe https://bbs.example C:\\WWW %%handle @C:\\NODE\\DSZ.CTL\n")
		fmt.Fprintf(os.Stderr, "  Linux:   ./webdownload https://bbs.example /var/www/bbs \"$LOGNAME\" @/node/dsz.ctl\n")
		fmt.Fprintf(os.Stderr, "  weburl is the public site, e.g. https://shsbbs.net\n")
		fmt.Fprintf(os.Stderr, "  A ctlfile starting with @ is a list of files (DSZ.CTL). Without @ it is one file.\n")
		fmt.Fprintf(os.Stderr, "  If omitted, DSZ.CTL beside DOOR32.SYS is used as a list.\n")
		return 2
	}

	base := normalizeBaseURL(args[0])
	webdir := strings.TrimSpace(args[1])
	user := strings.TrimSpace(args[2])
	if base == "" {
		fmt.Fprintf(os.Stderr, "webdownload: web address is required\n")
		return 2
	}

	var drop *DropFile
	var dropPath string
	if !*local {
		var err error
		dropPath, err = findDoor32(*door32Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "webdownload: %v\n", err)
			return 1
		}
		drop, err = readDoor32(dropPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "webdownload: %v\n", err)
			return 1
		}
	} else if *door32Path != "" {
		dropPath = *door32Path
	}

	spec := ctlArgFromArgs(args)

	if user == "" || user == "*" {
		if drop != nil && drop.HandleName != "" {
			user = drop.HandleName
		} else {
			user = "user"
		}
	}
	user = sanitizeUser(user)

	sess, err := openSession(*local, *useWS, drop)
	if err != nil {
		fmt.Fprintf(os.Stderr, "webdownload: session: %v\n", err)
		return 1
	}
	defer sess.Close()

	writeErr := func(msg string) {
		_, _ = sess.Write([]byte(ansiReset + ansiClear + ansiRed + "  " + msg + "\r\n\r\n" + ansiCyan + "  Press ESCAPE or Ctrl-X to return to the BBS.\r\n" + ansiReset))
	}
	pause := func() {
		var limit time.Duration
		if drop != nil {
			limit = drop.TimeLeft
		}
		fmt.Fprintf(os.Stderr, "webdownload: waiting for ESC\n")
		for {
			err := waitEscape(sess, limit)
			if err == nil {
				return
			}
			fmt.Fprintf(os.Stderr, "webdownload: wait for ESC: %v\n", err)
			if isDisconnect(err) {
				return
			}
			if ne, ok := err.(interface{ Timeout() bool }); ok && ne.Timeout() {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	_, _ = sess.Write(renderWorking())

	files, ctlPath, err := loadTaggedFiles(spec, dropPath)
	if err != nil {
		writeErr(err.Error())
		pause()
		return 1
	}

	total, err := totalCTLSize(files)
	if err != nil {
		writeErr(err.Error())
		pause()
		return 1
	}

	subdir, dest, err := makeDownloadDir(webdir, user)
	if err != nil {
		writeErr(err.Error())
		pause()
		return 1
	}
	if err := checkDestFree(dest); err != nil {
		_ = os.Remove(dest)
		writeErr(err.Error())
		pause()
		return 1
	}

	meter := &progressMeter{w: sess}
	copied, copyErrs := copyCTLFilesProgress(files, dest, total, meter.Update)
	if err := writeIndex(dest, base, user, subdir, copied); err != nil {
		copyErrs = append(copyErrs, err)
	}

	baud := 38400
	if drop != nil && drop.Baud > 0 {
		baud = drop.Baud
	}
	if err := appendDSZLog(copied, baud, ctlPath, dropPath); err != nil {
		fmt.Fprintf(os.Stderr, "webdownload: %v\n", err)
	}

	url := publicURL(base, user, subdir)
	fmt.Fprintf(os.Stderr, "webdownload: %s -> %s\n", dest, url)

	if _, err := sess.Write(renderScreen(user, url, copied, copyErrs)); err != nil {
		fmt.Fprintf(os.Stderr, "webdownload: write screen: %v\n", err)
		return 1
	}
	pause()
	return 0
}

func progName() string {
	name := filepath.Base(os.Args[0])
	if name == "" || name == "." {
		return "webdownload"
	}
	return name
}
