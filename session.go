package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

const (
	keyEscape = 0x1B
	keyCancel = 0x18 // Ctrl-X / Zmodem CAN
)

var errInvalidHandle = errors.New("invalid socket handle")

type session interface {
	io.ReadWriteCloser
}

type stdioSession struct {
	in  *os.File
	out *os.File
}

func (s stdioSession) Read(p []byte) (int, error)  { return s.in.Read(p) }
func (s stdioSession) Write(p []byte) (int, error) { return s.out.Write(p) }
func (s stdioSession) Close() error                { return nil }

func openSession(local, useWS bool, drop *DropFile) (session, error) {
	if local {
		return stdioSession{in: os.Stdin, out: os.Stdout}, nil
	}
	if drop != nil && drop.CommType == 1 {
		return nil, fmt.Errorf("serial (comm type 1) is not supported")
	}
	if drop != nil && drop.Handle != 0 {
		conn, err := connFromHandle(drop.Handle)
		if err != nil {
			return nil, fmt.Errorf("door32 socket %d: %w", drop.Handle, err)
		}
		if useWS {
			return newWSSession(conn)
		}
		return conn, nil
	}
	return stdioSession{in: os.Stdin, out: os.Stdout}, nil
}

func waitEscape(sess session, limit time.Duration) error {
	r := io.Reader(sess)
	if s, ok := sess.(*wsSession); ok {
		r = s.raw
	}
	if limit > 0 {
		type deadliner interface {
			SetReadDeadline(time.Time) error
		}
		if d, ok := r.(deadliner); ok {
			_ = d.SetReadDeadline(time.Now().Add(limit))
		}
	}
	return waitEscapeRaw(r)
}

func waitEscapeRaw(r io.Reader) error {
	buf := make([]byte, 256)
	for {
		n, err := r.Read(buf)
		if n > 0 && doneKey(buf[:n]) {
			return nil
		}
		if err == nil {
			if n == 0 {
				time.Sleep(20 * time.Millisecond)
			}
			continue
		}
		if isRetryableWait(err) {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		return err
	}
}

func doneKey(p []byte) bool {
	return bytes.IndexByte(p, keyEscape) >= 0 ||
		bytes.IndexByte(p, keyCancel) >= 0 ||
		bytes.IndexByte(p, 0x03) >= 0 // Ctrl-C
}
