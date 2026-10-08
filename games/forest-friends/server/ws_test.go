package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// maskedFrame builds a client frame (masked) with the given first header byte.
func maskedFrame(b0 byte, payload []byte) []byte {
	n := len(payload)
	buf := []byte{b0}
	switch {
	case n < 126:
		buf = append(buf, 0x80|byte(n))
	case n <= 0xFFFF:
		buf = append(buf, 0x80|126)
		buf = binary.BigEndian.AppendUint16(buf, uint16(n))
	default:
		buf = append(buf, 0x80|127)
		buf = binary.BigEndian.AppendUint64(buf, uint64(n))
	}
	mask := [4]byte{0x37, 0xfa, 0x21, 0x3d}
	buf = append(buf, mask[:]...)
	for i, b := range payload {
		buf = append(buf, b^mask[i%4])
	}
	return buf
}

func reader(b []byte) *bufio.Reader { return bufio.NewReader(bytes.NewReader(b)) }

func TestAcceptKey(t *testing.T) {
	// RFC 6455 section 1.3 sample.
	if got := acceptKey("dGhlIHNhbXBsZSBub25jZQ=="); got != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Fatalf("acceptKey = %q", got)
	}
}

func TestReadFrameMasked(t *testing.T) {
	for _, n := range []int{5, 200} {
		payload := bytes.Repeat([]byte("ab"), n)[:n]
		op, got, err := readFrame(reader(maskedFrame(0x81, payload)), 1024)
		if err != nil || op != opText || !bytes.Equal(got, payload) {
			t.Fatalf("n=%d: op=%x err=%v payload match=%v", n, op, err, bytes.Equal(got, payload))
		}
	}
}

func TestReadFramePing(t *testing.T) {
	op, got, err := readFrame(reader(maskedFrame(0x89, []byte("hi"))), 1024)
	if err != nil || op != opPing || string(got) != "hi" {
		t.Fatalf("op=%x payload=%q err=%v", op, got, err)
	}
}

func TestReadFrameErrors(t *testing.T) {
	unmasked := []byte{0x81, 0x02, 'h', 'i'}
	cases := []struct {
		name string
		data []byte
		code uint16
	}{
		{"unmasked", unmasked, 1002},
		{"rsv", maskedFrame(0xC1, []byte("x")), 1002},
		{"oversize", maskedFrame(0x81, make([]byte, 1025)), 1009},
		{"fragmented", maskedFrame(0x01, []byte("x")), 1003},
		{"continuation", maskedFrame(0x80, []byte("x")), 1003},
		{"binary", maskedFrame(0x82, []byte("x")), 1003},
		{"control too long", maskedFrame(0x89, make([]byte, 126)), 1002},
		{"unknown opcode", maskedFrame(0x83, []byte("x")), 1002},
	}
	for _, c := range cases {
		_, _, err := readFrame(reader(c.data), 1024)
		var we *wsError
		if !errors.As(err, &we) || we.code != c.code {
			t.Errorf("%s: err = %v, want code %d", c.name, err, c.code)
		}
	}
	full := maskedFrame(0x81, []byte("hello"))
	if _, _, err := readFrame(reader(full[:len(full)-2]), 1024); err == nil {
		t.Error("short read: want error")
	} else if errors.As(err, new(*wsError)) {
		t.Errorf("short read: got wsError %v, want I/O error", err)
	}
}

// recConn records every Write call.
type recConn struct {
	net.Conn
	writes [][]byte
}

func (c *recConn) Write(b []byte) (int, error) {
	c.writes = append(c.writes, append([]byte(nil), b...))
	return len(b), nil
}

func TestWriteFrame(t *testing.T) {
	cases := []struct {
		n      int
		header []byte
	}{
		{5, []byte{0x81, 5}},
		{200, []byte{0x81, 126, 0, 200}},
		{70000, []byte{0x81, 127, 0, 0, 0, 0, 0, 1, 0x11, 0x70}},
	}
	for _, c := range cases {
		rc := &recConn{}
		payload := bytes.Repeat([]byte{'x'}, c.n)
		if err := writeFrame(rc, opText, payload); err != nil {
			t.Fatal(err)
		}
		if len(rc.writes) != 1 {
			t.Fatalf("n=%d: %d writes, want 1", c.n, len(rc.writes))
		}
		w := rc.writes[0]
		if !bytes.Equal(w[:len(c.header)], c.header) || !bytes.Equal(w[len(c.header):], payload) {
			t.Errorf("n=%d: header % x, want % x (len %d)", c.n, w[:len(c.header)], c.header, len(w))
		}
	}
}

func TestCloseFrame(t *testing.T) {
	if got := closeFrame(1013, "bye"); !bytes.Equal(got, []byte{0x03, 0xF5, 'b', 'y', 'e'}) {
		t.Fatalf("closeFrame = % x", got)
	}
}

func validHandshake() *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Header.Set("Upgrade", "websocket")
	r.Header.Set("Connection", "keep-alive, Upgrade")
	r.Header.Set("Sec-WebSocket-Version", "13")
	r.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	return r
}

func TestUpgradeRejections(t *testing.T) {
	cases := map[string]func(r *http.Request){
		"method":     func(r *http.Request) { r.Method = http.MethodPost },
		"upgrade":    func(r *http.Request) { r.Header.Del("Upgrade") },
		"connection": func(r *http.Request) { r.Header.Set("Connection", "keep-alive") },
		"version":    func(r *http.Request) { r.Header.Set("Sec-WebSocket-Version", "12") },
		"key":        func(r *http.Request) { r.Header.Set("Sec-WebSocket-Key", "c2hvcnQ=") },
		"bad base64": func(r *http.Request) { r.Header.Set("Sec-WebSocket-Key", "!!!") },
	}
	for name, mutate := range cases {
		r := validHandshake()
		mutate(r)
		w := httptest.NewRecorder()
		if _, _, err := upgrade(w, r); err == nil || w.Code != http.StatusBadRequest {
			t.Errorf("%s: code %d err %v, want 400", name, w.Code, err)
		}
	}
	// A valid request on a ResponseWriter without Hijacker gets 500.
	w := httptest.NewRecorder()
	if _, _, err := upgrade(w, validHandshake()); err == nil || w.Code != http.StatusInternalServerError {
		t.Errorf("no hijacker: code %d err %v, want 500", w.Code, err)
	}
}
