package game

import "sort"

// ClientMsg is sent by a client: {"t":"move","x":1.2,"z":-3.4,"r":1.57}.
type ClientMsg struct {
	T string  `json:"t"` // "move"
	X float64 `json:"x"`
	Z float64 `json:"z"`
	R float64 `json:"r"`
}

// PlayerState is one player in a state message.
type PlayerState struct {
	ID     int     `json:"id"`
	Animal string  `json:"animal"`
	X      float64 `json:"x"`
	Z      float64 `json:"z"`
	R      float64 `json:"r"`
	Score  int     `json:"score"`
}

// StarState is one star in a state message.
type StarState struct {
	ID int     `json:"id"`
	X  float64 `json:"x"`
	Z  float64 `json:"z"`
}

// ServerMsg is sent by the server.
type ServerMsg struct {
	T       string        `json:"t"` // "welcome" | "state" | "full"
	ID      int           `json:"id,omitempty"`
	Players []PlayerState `json:"players,omitempty"`
	Stars   []StarState   `json:"stars,omitempty"`
}

// Snapshot returns a state message: players sorted by ID, stars in slice order.
func Snapshot(w *World) ServerMsg {
	msg := ServerMsg{T: "state"}
	for _, p := range w.Players {
		msg.Players = append(msg.Players, PlayerState{ID: p.ID, Animal: p.Animal, X: p.X, Z: p.Z, R: p.R, Score: p.Score})
	}
	sort.Slice(msg.Players, func(i, j int) bool { return msg.Players[i].ID < msg.Players[j].ID })
	for _, s := range w.Stars {
		msg.Stars = append(msg.Stars, StarState{ID: s.ID, X: s.X, Z: s.Z})
	}
	return msg
}
