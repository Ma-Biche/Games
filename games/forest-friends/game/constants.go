// Package game holds the rules shared by the client and the server.
package game

// Tunables. All distances are world units.
const (
	ArenaHalf    = 5.6  // player centre is clamped to |x|,|z| <= ArenaHalf
	PlayerRadius = 0.35 // collision radius of a player
	MoveSpeed    = 4.0  // units per second
	StarCount    = 5    // always exactly this many stars
	PickupRadius = 0.8  // player-centre-to-star distance
	TickHz       = 15   // server broadcast and client send rate
	MaxPlayers   = 8
	GroundY      = 0.05 // patch-grass flat top surface
	AnimalScale  = 0.6
)

// Animals lists the playable animals; each player gets the first unused one.
var Animals = []string{"fox", "bunny", "cat", "dog", "panda", "pig", "penguin", "chick"}
