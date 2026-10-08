package main

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Ma-Biche/Games/games/forest-friends/game"
)

const (
	maxFrame     = 1024
	writeTimeout = 5 * time.Second
)

type frame struct {
	op      byte
	payload []byte
}

type client struct {
	id       int
	animal   string
	conn     net.Conn
	send     chan frame // cap 16; sent to / closed only while holding Hub.mu
	lastMove time.Time
}

// Hub owns the World and every connected client.
type Hub struct {
	mu      sync.Mutex
	world   *game.World
	clients map[int]*client
}

// NewHub creates a hub with a fresh world.
func NewHub(seed uint64) *Hub {
	return &Hub{world: game.NewWorld(seed), clients: map[int]*client{}}
}

// Run ticks at TickHz: collects stars and broadcasts a snapshot.
// It returns when stop is closed (a nil stop runs forever).
func (h *Hub) Run(stop <-chan struct{}) {
	t := time.NewTicker(time.Second / game.TickHz)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			h.tick()
		}
	}
}

func (h *Hub) tick() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.world.CollectStars()
	msg, err := json.Marshal(game.Snapshot(h.world))
	if err != nil {
		log.Printf("hub: marshal state: %v", err)
		return
	}
	clients := make([]*client, 0, len(h.clients))
	for _, c := range h.clients {
		clients = append(clients, c)
	}
	for _, c := range clients {
		h.enqueueLocked(c, frame{opText, msg})
	}
}

// enqueueLocked queues f without blocking; a full queue drops the client. Requires mu.
func (h *Hub) enqueueLocked(c *client, f frame) bool {
	if h.clients[c.id] != c {
		return false
	}
	select {
	case c.send <- f:
		return true
	default:
		log.Printf("player %d (%s) too slow, dropping", c.id, c.animal)
		h.removeLocked(c)
		return false
	}
}

// enqueue locks mu and queues f; false if c was already removed.
func (h *Hub) enqueue(c *client, f frame) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.enqueueLocked(c, f)
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.removeLocked(c)
}

// removeLocked is idempotent. Requires mu.
func (h *Hub) removeLocked(c *client) {
	if h.clients[c.id] != c {
		return
	}
	delete(h.clients, c.id)
	h.world.RemovePlayer(c.id)
	close(c.send)
	log.Printf("player %d (%s) left", c.id, c.animal)
}

// writer is the only goroutine writing to a registered client's conn.
func (h *Hub) writer(c *client) {
	for f := range c.send {
		c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		if writeFrame(c.conn, f.op, f.payload) != nil {
			break
		}
	}
	c.conn.Close()
}

// ServeWS upgrades the request, joins the player and runs its reader loop.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, br, err := upgrade(w, r)
	if err != nil {
		return
	}

	h.mu.Lock()
	p, err := h.world.AddPlayer()
	if err != nil {
		h.mu.Unlock()
		full, _ := json.Marshal(game.ServerMsg{T: "full"})
		conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		writeFrame(conn, opText, full)
		writeFrame(conn, opClose, closeFrame(1013, ""))
		conn.Close()
		log.Printf("ws: full, rejected %s", r.RemoteAddr)
		return
	}
	c := &client{id: p.ID, animal: p.Animal, conn: conn, send: make(chan frame, 16), lastMove: time.Now()}
	h.clients[c.id] = c
	welcome, _ := json.Marshal(game.ServerMsg{T: "welcome", ID: c.id})
	state, _ := json.Marshal(game.Snapshot(h.world))
	h.enqueueLocked(c, frame{opText, welcome})
	h.enqueueLocked(c, frame{opText, state})
	log.Printf("player %d (%s) joined", c.id, c.animal)
	h.mu.Unlock()
	go h.writer(c)

	for {
		op, payload, err := readFrame(br, maxFrame)
		if err != nil {
			var we *wsError
			if errors.As(err, &we) {
				h.enqueue(c, frame{opClose, closeFrame(we.code, "")})
				log.Printf("ws: player %d protocol error %d", c.id, we.code)
			}
			h.remove(c)
			return
		}
		switch op {
		case opText:
			var msg game.ClientMsg
			if err := json.Unmarshal(payload, &msg); err != nil {
				log.Printf("ws: player %d bad message: %v", c.id, err)
				continue
			}
			if msg.T != "move" {
				continue
			}
			if !h.move(c, msg) {
				return
			}
		case opPing:
			if !h.enqueue(c, frame{opPong, payload}) {
				return
			}
		case opPong:
		case opClose:
			h.enqueue(c, frame{opClose, closeFrame(1000, "")})
			h.remove(c)
			return
		}
	}
}

// move applies a client move under mu; false if c was already removed.
func (h *Hub) move(c *client, msg game.ClientMsg) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c.id] != c {
		return false
	}
	now := time.Now()
	elapsed := min(now.Sub(c.lastMove), time.Second)
	maxDist := min(game.MoveSpeed*1.25*elapsed.Seconds()+0.25, 2.0)
	h.world.ApplyMove(c.id, msg.X, msg.Z, msg.R, maxDist)
	c.lastMove = now
	return true
}
