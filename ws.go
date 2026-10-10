package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// discardHandshakeConn swallows the HTTP 101 written by gorilla's Upgrader so
// we can attach a *websocket.Conn to a Door32 socket whose handshake is done.
type discardHandshakeConn struct {
	net.Conn
	buf       []byte
	discarded bool
}

func (c *discardHandshakeConn) Write(p []byte) (int, error) {
	if c.discarded {
		return c.Conn.Write(p)
	}
	c.buf = append(c.buf, p...)
	if i := bytes.Index(c.buf, []byte("\r\n\r\n")); i >= 0 {
		rest := append([]byte(nil), c.buf[i+4:]...)
		c.buf = nil
		c.discarded = true
		if len(rest) > 0 {
			if _, err := c.Conn.Write(rest); err != nil {
				return len(p), err
			}
		}
	}
	return len(p), nil
}

type wsHijacker struct {
	header http.Header
	conn   net.Conn
}

func (h *wsHijacker) Header() http.Header {
	if h.header == nil {
		h.header = make(http.Header)
	}
	return h.header
}

func (h *wsHijacker) Write([]byte) (int, error) { return 0, nil }
func (h *wsHijacker) WriteHeader(int)           {}

func (h *wsHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	wrapped := &discardHandshakeConn{Conn: h.conn}
	br := bufio.NewReaderSize(h.conn, 4096)
	bw := bufio.NewWriterSize(wrapped, 4096)
	return wrapped, bufio.NewReadWriter(br, bw), nil
}

func wrapWebSocket(raw net.Conn) (*websocket.Conn, error) {
	req := httptest.NewRequest(http.MethodGet, "/bbs", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     func(*http.Request) bool { return true },
	}
	ws, err := upgrader.Upgrade(&wsHijacker{conn: raw}, req, nil)
	if err != nil {
		return nil, fmt.Errorf("websocket wrap: %w", err)
	}
	return ws, nil
}

// wsSession presents gorilla/websocket frames as a byte stream for ANSI I/O.
type wsSession struct {
	ws   *websocket.Conn
	mu   sync.Mutex
	buf  []byte
	raw  net.Conn
}

func newWSSession(raw net.Conn) (*wsSession, error) {
	ws, err := wrapWebSocket(raw)
	if err != nil {
		return nil, err
	}
	return &wsSession{ws: ws, raw: raw}, nil
}

func (s *wsSession) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Binary frames keep CP437 and ANSI CSI sequences byte-accurate.
	if err := s.ws.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (s *wsSession) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for len(s.buf) == 0 {
		mt, data, err := s.ws.ReadMessage()
		if err != nil {
			return 0, err
		}
		switch mt {
		case websocket.TextMessage, websocket.BinaryMessage:
			s.buf = data
		default:
			continue
		}
	}
	n := copy(p, s.buf)
	s.buf = s.buf[n:]
	return n, nil
}

func (s *wsSession) Close() error {
	// Do not send a close frame; EleBBS still owns the live session.
	return s.raw.Close()
}

func (s *wsSession) SetReadDeadline(t time.Time) error {
	return s.ws.SetReadDeadline(t)
}

func (s *wsSession) SetWriteDeadline(t time.Time) error {
	return s.ws.SetWriteDeadline(t)
}
