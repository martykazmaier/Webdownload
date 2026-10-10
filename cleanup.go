package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const expireAfter = 24 * time.Hour

type expiredDir struct {
	Path string
	Age  time.Duration
}

func cleanupUserDir(webdir, user string, now time.Time, maxAge time.Duration) (removed []expiredDir, errs []error) {
	return expireSubdirs(userWebfilesDir(webdir, user), now, maxAge)
}

func cleanupAllUsers(webdir string, now time.Time, maxAge time.Duration) (removed []expiredDir, errs []error) {
	root := filepath.Join(filepath.Clean(webdir), webfilesDir)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []error{fmt.Errorf("cleanup %s: %w", root, err)}
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		childRemoved, childErrs := expireSubdirs(filepath.Join(root, e.Name()), now, maxAge)
		removed = append(removed, childRemoved...)
		errs = append(errs, childErrs...)
	}
	return removed, errs
}

func runCleanup(webdir, user string) int {
	now := time.Now()
	var removed []expiredDir
	var errs []error
	if user == "" {
		fmt.Fprintf(os.Stderr, "webdownload: cleanup %s\\%s\\* older than %s\n", webdir, webfilesDir, expireAfter)
		removed, errs = cleanupAllUsers(webdir, now, expireAfter)
	} else {
		user = sanitizeUser(user)
		fmt.Fprintf(os.Stderr, "webdownload: cleanup %s\\%s\\%s older than %s\n", webdir, webfilesDir, user, expireAfter)
		removed, errs = cleanupUserDir(webdir, user, now, expireAfter)
	}
	for _, d := range removed {
		fmt.Fprintf(os.Stderr, "webdownload: expired %s (%s old)\n", d.Path, d.Age.Round(time.Minute))
	}
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "webdownload: cleanup: %v\n", e)
	}
	if len(errs) > 0 {
		return 1
	}
	fmt.Fprintf(os.Stderr, "webdownload: cleanup removed %d directories\n", len(removed))
	return 0
}

func expireSubdirs(parent string, now time.Time, maxAge time.Duration) (removed []expiredDir, errs []error) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []error{fmt.Errorf("cleanup %s: %w", parent, err)}
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(parent, e.Name())
		gone, err := expireDirIfOld(path, now, maxAge)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if gone != nil {
			removed = append(removed, *gone)
		}
	}
	return removed, errs
}

func expireDirIfOld(path string, now time.Time, maxAge time.Duration) (*expiredDir, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	if !st.IsDir() {
		return nil, nil
	}
	age := now.Sub(st.ModTime())
	if age <= maxAge {
		return nil, nil
	}
	if err := os.RemoveAll(path); err != nil {
		return nil, fmt.Errorf("delete %s: %w", path, err)
	}
	return &expiredDir{Path: path, Age: age}, nil
}
