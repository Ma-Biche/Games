package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
)

// wsError is a WebSocket protocol error; code is the close code to send.
type wsError struct {
	code uint16
	msg  string
}

func (e *wsError) Error() string { return fmt.Sprintf("ws: %s (%d)", e.msg, e.code) }

// WebSocket opcodes.
const (
	opText  = 0x1
	opClose = 0x8
	opPing  = 0x9
	opPong  = 0xA
)

// acceptKey returns the Sec-WebSocket-Accept value for a client key.
func acceptKey(key string) string {
	h := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h[:])
}

// hasToken reports whether a comma-separated header value contains token (case-insensitive).
func hasToken(value, token string) bool {
	for _, part := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}

// handshakeError returns why r is not a valid WebSocket upgrade request, or "".
func handshakeError(r *http.Request) string {
	if r.Method != http.MethodGet {
		return "method must be GET"
	}
	if !strings.Contains(strings.ToLower(r.Header.Get("Upgrade")), "websocket") {
		return "missing Upgrade: websocket"
	}
	if !hasToken(r.Header.Get("Connection"), "upgrade") {
		return "missing Connection: upgrade"
	}
	if r.Header.Get("Sec-WebSocket-Version") != "13" {
		return "Sec-WebSocket-Version must be 13"
	}
	key, err := base64.StdEncoding.DecodeString(r.Header.Get("Sec-WebSocket-Key"))
	if err != nil || len(key) != 16 {
		return "bad Sec-WebSocket-Key"
	}
	return ""
}

// upgrade performs the server side of the RFC 6455 handshake.
func upgrade(w http.ResponseWriter, r *http.Request) (net.Conn, *bufio.Reader, error) {
	if reason := handshakeError(r); reason != "" {
		http.Error(w, reason, http.StatusBadRequest)
		log.Printf("ws: rejected %s: %s", r.RemoteAddr, reason)
		return nil, nil, errors.New(reason)
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket not supported", http.StatusInternalServerError)
		log.Printf("ws: rejected %s: no hijacker", r.RemoteAddr)
		return nil, nil, errors.New("no hijacker")
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		log.Printf("ws: rejected %s: hijack: %v", r.RemoteAddr, err)
		return nil, nil, err
	}
	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey(r.Header.Get("Sec-WebSocket-Key")) + "\r\n\r\n"
	if _, err := rw.WriteString(resp); err != nil {
		conn.Close()
		return nil, nil, err
	}
	if err := rw.Flush(); err != nil {
		conn.Close()
		return nil, nil, err
	}
	return conn, rw.Reader, nil
}

// readFrame reads one client frame and returns its opcode and unmasked payload.
// It only parses; it never writes to the network.
func readFrame(br *bufio.Reader, max int) (op byte, payload []byte, err error) {
	var hdr [2]byte
	if _, err := io.ReadFull(br, hdr[:]); err != nil {
		return 0, nil, err
	}
	fin := hdr[0]&0x80 != 0
	op = hdr[0] & 0x0F
	if hdr[0]&0x70 != 0 {
		return 0, nil, &wsError{1002, "RSV bit set"}
	}
	if hdr[1]&0x80 == 0 {
		return 0, nil, &wsError{1002, "unmasked frame"}
	}
	if !fin || op == 0x0 {
		return 0, nil, &wsError{1003, "fragmentation unsupported"}
	}
	if op == 0x2 {
		return 0, nil, &wsError{1003, "binary unsupported"}
	}
	control := op >= opClose && op <= opPong
	if op != opText && !control {
		return 0, nil, &wsError{1002, "unknown opcode"}
	}

	n := uint64(hdr[1] & 0x7F)
	switch n {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(br, ext[:]); err != nil {
			return 0, nil, err
		}
		n = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(br, ext[:]); err != nil {
			return 0, nil, err
		}
		n = binary.BigEndian.Uint64(ext[:])
	}
	if control && n > 125 {
		return 0, nil, &wsError{1002, "control frame too long"}
	}
	if n > uint64(max) {
		return 0, nil, &wsError{1009, "frame too long"}
	}

	var mask [4]byte
	if _, err := io.ReadFull(br, mask[:]); err != nil {
		return 0, nil, err
	}
	payload = make([]byte, n)
	if _, err := io.ReadFull(br, payload); err != nil {
		return 0, nil, err
	}
	for i := range payload {
		payload[i] ^= mask[i%4]
	}
	return op, payload, nil
}

// writeFrame writes one unmasked, final frame with a single conn.Write.
func writeFrame(conn net.Conn, op byte, payload []byte) error {
	n := len(payload)
	buf := make([]byte, 0, n+10)
	buf = append(buf, 0x80|op)
	switch {
	case n < 126:
		buf = append(buf, byte(n))
	case n <= 0xFFFF:
		buf = append(buf, 126)
		buf = binary.BigEndian.AppendUint16(buf, uint16(n))
	default:
		buf = append(buf, 127)
		buf = binary.BigEndian.AppendUint64(buf, uint64(n))
	}
	buf = append(buf, payload...)
	_, err := conn.Write(buf)
	return err
}

// closeFrame returns a close payload: 2-byte big-endian code + reason.
func closeFrame(code uint16, reason string) []byte {
	return append(binary.BigEndian.AppendUint16(nil, code), reason...)
}
