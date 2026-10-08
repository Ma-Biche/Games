package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Ma-Biche/Games/games/forest-friends/game"
)

// testClient is a raw-TCP WebSocket client.
type testClient struct {
	conn net.Conn
	br   *bufio.Reader
}

func startHub(t *testing.T) *httptest.Server {
	t.Helper()
	h := NewHub(1)
	stop := make(chan struct{})
	go h.Run(stop)
	srv := httptest.NewServer(http.HandlerFunc(h.ServeWS))
	t.Cleanup(func() {
		srv.Close()
		close(stop)
	})
	return srv
}

func dial(t *testing.T, srv *httptest.Server) *testClient {
	t.Helper()
	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	key := "dGhlIHNhbXBsZSBub25jZQ=="
	req := "GET /ws HTTP/1.1\r\nHost: test\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Version: 13\r\nSec-WebSocket-Key: " + key + "\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols || resp.Header.Get("Sec-WebSocket-Accept") != acceptKey(key) {
		t.Fatalf("handshake: %s accept=%q", resp.Status, resp.Header.Get("Sec-WebSocket-Accept"))
	}
	return &testClient{conn: conn, br: br}
}

// next reads one unmasked server frame.
func (c *testClient) next() (byte, []byte, error) {
	var hdr [2]byte
	if _, err := io.ReadFull(c.br, hdr[:]); err != nil {
		return 0, nil, err
	}
	n := uint64(hdr[1] & 0x7F)
	switch n {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(c.br, ext[:]); err != nil {
			return 0, nil, err
		}
		n = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(c.br, ext[:]); err != nil {
			return 0, nil, err
		}
		n = binary.BigEndian.Uint64(ext[:])
	}
	payload := make([]byte, n)
	_, err := io.ReadFull(c.br, payload)
	return hdr[0] & 0x0F, payload, err
}

func (c *testClient) send(t *testing.T, b0 byte, payload []byte) {
	t.Helper()
	if _, err := c.conn.Write(maskedFrame(b0, payload)); err != nil {
		t.Fatal(err)
	}
}

// readFrameUntil skips frames until pred matches or the timeout expires.
func readFrameUntil(t *testing.T, c *testClient, pred func(op byte, payload []byte) bool, timeout time.Duration) (byte, []byte) {
	t.Helper()
	c.conn.SetReadDeadline(time.Now().Add(timeout))
	defer c.conn.SetReadDeadline(time.Time{})
	for {
		op, payload, err := c.next()
		if err != nil {
			t.Fatalf("waiting for frame: %v", err)
		}
		if pred(op, payload) {
			return op, payload
		}
	}
}

// readUntil skips frames until a text message matching pred arrives.
func readUntil(t *testing.T, c *testClient, pred func(game.ServerMsg) bool, timeout time.Duration) game.ServerMsg {
	t.Helper()
	var msg game.ServerMsg
	readFrameUntil(t, c, func(op byte, payload []byte) bool {
		if op != opText {
			return false
		}
		msg = game.ServerMsg{}
		return json.Unmarshal(payload, &msg) == nil && pred(msg)
	}, timeout)
	return msg
}

func isWelcome(m game.ServerMsg) bool { return m.T == "welcome" }

func selfIn(m game.ServerMsg, id int) (game.PlayerState, bool) {
	for _, p := range m.Players {
		if p.ID == id {
			return p, true
		}
	}
	return game.PlayerState{}, false
}

func TestHubWelcomeAndState(t *testing.T) {
	srv := startHub(t)
	c1 := dial(t, srv)
	if w := readUntil(t, c1, isWelcome, 2*time.Second); w.ID != 1 {
		t.Fatalf("first welcome id = %d, want 1", w.ID)
	}
	c2 := dial(t, srv)
	if w := readUntil(t, c2, isWelcome, 2*time.Second); w.ID != 2 {
		t.Fatalf("second welcome id = %d, want 2", w.ID)
	}
	for _, c := range []*testClient{c1, c2} {
		readUntil(t, c, func(m game.ServerMsg) bool {
			return m.T == "state" && len(m.Players) == 2 && len(m.Stars) == game.StarCount
		}, 2*time.Second)
	}
}

func TestHubMove(t *testing.T) {
	srv := startHub(t)
	c := dial(t, srv)
	id := readUntil(t, c, isWelcome, 2*time.Second).ID
	var start game.PlayerState
	readUntil(t, c, func(m game.ServerMsg) bool {
		var ok bool
		start, ok = selfIn(m, id)
		return m.T == "state" && ok
	}, 2*time.Second)

	// A 0.1 step is within the minimum budget (0.25), so only Resolve applies.
	wantX, wantZ := game.Resolve(start.X+0.1, start.Z)
	move, _ := json.Marshal(game.ClientMsg{T: "move", X: start.X + 0.1, Z: start.Z, R: 1.25})
	c.send(t, 0x81, move)
	readUntil(t, c, func(m game.ServerMsg) bool {
		p, ok := selfIn(m, id)
		return m.T == "state" && ok && p.R == 1.25 && p.X == wantX && p.Z == wantZ
	}, 2*time.Second)
}

func TestHubPing(t *testing.T) {
	srv := startHub(t)
	c := dial(t, srv)
	readUntil(t, c, isWelcome, 2*time.Second)
	c.send(t, 0x89, []byte("hi"))
	readFrameUntil(t, c, func(op byte, payload []byte) bool {
		return op == opPong && string(payload) == "hi"
	}, 2*time.Second)
}

func TestHubFull(t *testing.T) {
	srv := startHub(t)
	for i := 1; i <= game.MaxPlayers; i++ {
		c := dial(t, srv)
		if w := readUntil(t, c, isWelcome, 2*time.Second); w.ID != i {
			t.Fatalf("welcome id = %d, want %d", w.ID, i)
		}
	}
	c := dial(t, srv)
	c.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	op, payload, err := c.next()
	if err != nil || op != opText || string(payload) != `{"t":"full"}` {
		t.Fatalf("9th client first frame: op=%x payload=%q err=%v", op, payload, err)
	}
	op, payload, err = c.next()
	if err != nil || op != opClose || len(payload) < 2 || binary.BigEndian.Uint16(payload) != 1013 {
		t.Fatalf("9th client second frame: op=%x payload=% x err=%v", op, payload, err)
	}
}

func TestHubLeave(t *testing.T) {
	srv := startHub(t)
	c1 := dial(t, srv)
	readUntil(t, c1, isWelcome, 2*time.Second)
	c2 := dial(t, srv)
	readUntil(t, c2, isWelcome, 2*time.Second)
	readUntil(t, c1, func(m game.ServerMsg) bool { return m.T == "state" && len(m.Players) == 2 }, 2*time.Second)

	c2.send(t, 0x88, closeFrame(1000, ""))
	readFrameUntil(t, c2, func(op byte, _ []byte) bool { return op == opClose }, 2*time.Second)
	c2.conn.Close()

	readUntil(t, c1, func(m game.ServerMsg) bool { return m.T == "state" && len(m.Players) == 1 }, 2*time.Second)
}
