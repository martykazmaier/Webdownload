package main

import (
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

var (
	errInvalidHandle = errors.New("invalid socket handle")
	errIdleTimeout   = errors.New("idle timeout")
	errTimeExpired   = errors.New("time expired")
)

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

type deadliner interface {
	SetReadDeadline(time.Time) error
}

func waitEscape(sess session, limit time.Duration) error {
	return waitForExit(sess, limit, 0)
}

func waitForExit(sess session, sessionLeft, idle time.Duration) error {
	r := io.Reader(sess)
	if s, ok := sess.(*wsSession); ok {
		r = s.raw
	}
	d, hasDL := r.(deadliner)

	var sessionEnd time.Time
	if sessionLeft > 0 {
		sessionEnd = time.Now().Add(sessionLeft)
	}
	nextDeadline := func() time.Time {
		idleEnd := time.Time{}
		if idle > 0 {
			idleEnd = time.Now().Add(idle)
		}
		switch {
		case idleEnd.IsZero():
			return sessionEnd
		case sessionEnd.IsZero() || idleEnd.Before(sessionEnd):
			return idleEnd
		default:
			return sessionEnd
		}
	}

	deadline := nextDeadline()
	buf := make([]byte, 256)
	iac := 0
	for {
		if hasDL {
			_ = d.SetReadDeadline(deadline)
		}
		n, err := r.Read(buf)
		now := time.Now()
		if n > 0 {
			act, done := inspectInput(buf[:n], &iac)
			if done {
				if hasDL {
					_ = d.SetReadDeadline(time.Time{})
				}
				return nil
			}
			if act {
				deadline = nextDeadline()
			}
			if err == nil {
				continue
			}
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
		if ne, ok := err.(interface{ Timeout() bool }); ok && ne.Timeout() {
			if !sessionEnd.IsZero() && !now.Before(sessionEnd) {
				return errTimeExpired
			}
			if idle > 0 {
				return errIdleTimeout
			}
			return err
		}
		return err
	}
}

func waitEscapeRaw(r io.Reader) error {
	return waitForExit(rawSession{r}, 0, 0)
}

type rawSession struct{ r io.Reader }

func (s rawSession) Read(p []byte) (int, error)  { return s.r.Read(p) }
func (rawSession) Write([]byte) (int, error)     { return 0, nil }
func (rawSession) Close() error                  { return nil }

func inspectInput(p []byte, st *int) (activity, done bool) {
	for _, b := range p {
		switch *st {
		case 1:
			switch b {
			case 255:
				*st = 0
				activity = true
			case 250:
				*st = 3
			case 251, 252, 253, 254:
				*st = 2
			default:
				*st = 0
			}
		case 2:
			*st = 0
		case 3:
			if b == 255 {
				*st = 4
			}
		case 4:
			if b == 240 {
				*st = 0
			} else {
				*st = 3
			}
		default:
			if b == 255 {
				*st = 1
				continue
			}
			if b == 0 {
				continue
			}
			activity = true
			if b == keyEscape || b == keyCancel || b == 0x03 {
				done = true
			}
		}
	}
	return activity, done
}

func doneKey(p []byte) bool {
	st := 0
	_, done := inspectInput(p, &st)
	return done
}

func hangupSession(sess session) {
	if h, ok := sess.(interface{ Hangup() error }); ok {
		_ = h.Hangup()
		return
	}
	_ = sess.Close()
}
