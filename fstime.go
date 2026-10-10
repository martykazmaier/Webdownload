package main

import (
	"fmt"
	"os"
	"time"
)

const fsOpTimeout = 2 * time.Second

func timedOpen(path string) (*os.File, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}
	type result struct {
		f   *os.File
		err error
	}
	ch := make(chan result, 1)
	go func() {
		f, err := openSource(path)
		ch <- result{f, err}
	}()
	select {
	case r := <-ch:
		return r.f, r.err
	case <-time.After(fsOpTimeout):
		return nil, fmt.Errorf("timeout opening %s", path)
	}
}
