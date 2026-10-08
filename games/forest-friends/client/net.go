//go:build js && wasm

package main

import (
	"encoding/json"
	"math"
	"strings"
	"syscall/js"

	"github.com/Ma-Biche/Games/games/forest-friends/game"
)

const (
	modeConnecting = iota
	modeOnline
	modeOffline
)

// shown is the display state of a remote player, kept across state messages.
type shown struct {
	dispX, dispZ float64 // smoothed position actually drawn
	srvX, srvZ   float64 // latest server position
	r            float64 // latest server rotation (radians)
	animal       string
}

var (
	mode        = modeConnecting
	selfID      int
	synced      bool
	posX, posZ  float64 // local predicted position
	rot         float64 // local rotation (radians)
	remote      = map[int]*shown{}
	lastPlayers []game.PlayerState
	onlineStars []game.StarState
	selfAnimal  string
	selfScore   int

	world     *game.World // offline only
	ws        = js.Null()
	timeoutID = js.Null()

	lastSendMs          float64
	sentX, sentZ, sentR float64
)

func netConnect() {
	loc := js.Global().Get("location")
	proto := loc.Get("protocol").String()
	if proto == "file:" || strings.Contains(loc.Get("search").String(), "offline=1") {
		goOffline("start")
		return
	}
	scheme := "ws:"
	if proto == "https:" {
		scheme = "wss:"
	}
	mode = modeConnecting
	setStatus("connecting", "Connecting…")
	ws = js.Global().Get("WebSocket").New(scheme + "//" + loc.Get("host").String() + "/ws")
	on(ws, "message", onMessage)
	onFail := func(js.Value) {
		if mode == modeOffline {
			return
		}
		if mode == modeOnline {
			goOffline("lost")
		} else {
			goOffline("start")
		}
	}
	on(ws, "error", onFail)
	on(ws, "close", onFail)

	var timeout js.Func
	timeout = js.FuncOf(func(this js.Value, args []js.Value) any {
		timeout.Release()
		timeoutID = js.Null()
		if mode == modeConnecting {
			goOffline("start")
		}
		return nil
	})
	timeoutID = js.Global().Call("setTimeout", timeout, 3000)
}

func clearConnectTimeout() {
	if !timeoutID.IsNull() {
		js.Global().Call("clearTimeout", timeoutID)
		timeoutID = js.Null()
	}
}

func onMessage(e js.Value) {
	if mode == modeOffline {
		return
	}
	var msg game.ServerMsg
	if err := json.Unmarshal([]byte(e.Get("data").String()), &msg); err != nil {
		consoleCall("warn", "bad server message: "+err.Error())
		return
	}
	switch msg.T {
	case "welcome":
		selfID = msg.ID
		mode = modeOnline
		clearConnectTimeout()
		setStatus("online", "Online")
	case "state":
		if selfID == 0 {
			return
		}
		onState(msg)
	case "full":
		goOffline("full")
	}
}

func onState(msg game.ServerMsg) {
	present := map[int]bool{}
	for _, p := range msg.Players {
		present[p.ID] = true
		if p.ID == selfID {
			continue
		}
		if s, ok := remote[p.ID]; ok {
			s.srvX, s.srvZ, s.r, s.animal = p.X, p.Z, p.R, p.Animal
		} else {
			remote[p.ID] = &shown{dispX: p.X, dispZ: p.Z, srvX: p.X, srvZ: p.Z, r: p.R, animal: p.Animal}
		}
	}
	for id := range remote {
		if !present[id] {
			delete(remote, id)
		}
	}
	onlineStars = msg.Stars
	lastPlayers = msg.Players

	for _, p := range msg.Players {
		if p.ID != selfID {
			continue
		}
		if !synced {
			posX, posZ, rot = p.X, p.Z, p.R
			sentX, sentZ, sentR = p.X, p.Z, p.R
			synced = true
			focusX, focusZ = posX, posZ
		} else if math.Hypot(p.X-posX, p.Z-posZ) > 1.5 {
			posX, posZ = p.X, p.Z
		}
		selfScore, selfAnimal = p.Score, p.Animal
	}
}

// maybeSend sends the local position at most TickHz times per second, and
// only when it changed.
func maybeSend() {
	if mode != modeOnline || !synced || ws.Get("readyState").Int() != 1 {
		return
	}
	now := js.Global().Get("performance").Call("now").Float()
	if now-lastSendMs < 1000/game.TickHz {
		return
	}
	if math.Abs(posX-sentX) <= 0.001 && math.Abs(posZ-sentZ) <= 0.001 && math.Abs(rot-sentR) <= 0.001 {
		return
	}
	b, _ := json.Marshal(game.ClientMsg{T: "move", X: posX, Z: posZ, R: rot})
	ws.Call("send", string(b))
	lastSendMs = now
	sentX, sentZ, sentR = posX, posZ, rot
}

// goOffline switches to the local single-player world. It is idempotent.
func goOffline(reason string) {
	if mode == modeOffline {
		return
	}
	carry := mode == modeOnline && synced
	mode = modeOffline
	clearConnectTimeout()
	if !ws.IsNull() && ws.Get("readyState").Int() < 2 {
		ws.Call("close")
	}

	world = game.NewWorld(uint64(js.Global().Get("Date").Call("now").Float()))
	p, _ := world.AddPlayer()
	if carry {
		p.Animal, p.X, p.Z, p.R, p.Score = selfAnimal, posX, posZ, rot, selfScore
	} else {
		posX, posZ, rot = p.X, p.Z, p.R
	}
	selfID = p.ID
	clear(remote)
	lastPlayers = nil

	switch reason {
	case "lost":
		setStatus("offline", "Disconnected — playing offline")
		consoleCall("warn", "Forest Friends: connection lost, playing offline")
	case "full":
		setStatus("offline", "Server full — playing offline")
		consoleCall("info", "Forest Friends: server full, playing offline")
	default:
		setStatus("offline", "Offline — single player")
		consoleCall("info", "Forest Friends: offline, single player")
	}
}
