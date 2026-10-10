//go:build windows

package main

import (
	"io"
	"net"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	soSndTimeo      = 0x1005
	soRcvTimeo      = 0x1006
	fionbio         = 0x8004667E
	wsaEWOULDBLOCK  = 10035
	wsaEINTR        = 10004
	wsaEINPROGRESS  = 10036
	wsaETIMEDOUT    = 10060
	wsaECONNRESET   = 10054
	wsaECONNABORTED = 10053
)

var (
	ws2             = windows.NewLazySystemDLL("ws2_32.dll")
	procRecv        = ws2.NewProc("recv")
	procSend        = ws2.NewProc("send")
	procIoctlsocket = ws2.NewProc("ioctlsocket")
	procWSAGetLastError = ws2.NewProc("WSAGetLastError")
)

type sockAddr struct{}

func (sockAddr) Network() string { return "tcp" }
func (sockAddr) String() string  { return "door32" }

type winSocket struct {
	h      windows.Handle
	mu     sync.Mutex
	closed bool
}

func connFromHandle(handle uintptr) (net.Conn, error) {
	if handle == 0 || handle == uintptr(syscall.InvalidHandle) {
		return nil, errInvalidHandle
	}
	s := &winSocket{h: windows.Handle(handle)}
	_ = setNonBlocking(s.h, false)
	_ = windows.SetsockoptInt(s.h, windows.IPPROTO_TCP, windows.TCP_NODELAY, 1)
	return s, nil
}

func setNonBlocking(h windows.Handle, nb bool) error {
	var mode uint32
	if nb {
		mode = 1
	}
	r1, _, e := procIoctlsocket.Call(uintptr(h), uintptr(fionbio), uintptr(unsafe.Pointer(&mode)))
	if r1 != 0 {
		if e != syscall.Errno(0) {
			return e
		}
		return syscall.EINVAL
	}
	return nil
}

func lastSockErr(callErr error) syscall.Errno {
	if errno, ok := callErr.(syscall.Errno); ok && errno != 0 {
		return errno
	}
	r1, _, _ := procWSAGetLastError.Call()
	return syscall.Errno(r1)
}

func (c *winSocket) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	for {
		r1, _, e := procRecv.Call(uintptr(c.h), uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 0)
		n := int32(r1)
		if n == -1 {
			errno := lastSockErr(e)
			switch errno {
			case wsaEWOULDBLOCK, wsaEINTR, wsaEINPROGRESS:
				_ = setNonBlocking(c.h, false)
				time.Sleep(20 * time.Millisecond)
				continue
			case wsaETIMEDOUT:
				return 0, osTimeoutError{}
			default:
				return 0, errno
			}
		}
		if n == 0 {
			return 0, io.EOF
		}
		return int(n), nil
	}
}

func (c *winSocket) Write(b []byte) (int, error) {
	sent := 0
	for sent < len(b) {
		chunk := b[sent:]
		r1, _, e := procSend.Call(uintptr(c.h), uintptr(unsafe.Pointer(&chunk[0])), uintptr(len(chunk)), 0)
		n := int32(r1)
		if n == -1 {
			errno := lastSockErr(e)
			switch errno {
			case wsaEWOULDBLOCK, wsaEINTR, wsaEINPROGRESS:
				_ = setNonBlocking(c.h, false)
				time.Sleep(20 * time.Millisecond)
				continue
			default:
				return sent, errno
			}
		}
		if n == 0 {
			return sent, io.ErrShortWrite
		}
		sent += int(n)
	}
	return sent, nil
}

func (c *winSocket) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *winSocket) LocalAddr() net.Addr  { return sockAddr{} }
func (c *winSocket) RemoteAddr() net.Addr { return sockAddr{} }

func (c *winSocket) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}

func (c *winSocket) SetReadDeadline(t time.Time) error {
	return setSockTimeo(c.h, soRcvTimeo, t)
}

func (c *winSocket) SetWriteDeadline(t time.Time) error {
	return setSockTimeo(c.h, soSndTimeo, t)
}

func setSockTimeo(h windows.Handle, opt int, t time.Time) error {
	var ms int
	if !t.IsZero() {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}
		ms = int(d / time.Millisecond)
		if ms == 0 && d > 0 {
			ms = 1
		}
	}
	return windows.SetsockoptInt(h, windows.SOL_SOCKET, opt, ms)
}

type osTimeoutError struct{}

func (osTimeoutError) Error() string   { return "i/o timeout" }
func (osTimeoutError) Timeout() bool   { return true }
func (osTimeoutError) Temporary() bool { return true }
