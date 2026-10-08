package game

import (
	"errors"
	"math"
	"math/rand/v2"
)

// ErrFull is returned by AddPlayer when MaxPlayers are already in the world.
var ErrFull = errors.New("game full")

// Player is one animal in the arena. R is the facing angle in radians.
type Player struct {
	ID      int
	Animal  string
	X, Z, R float64
	Score   int
}

// Star is a collectible.
type Star struct {
	ID   int
	X, Z float64
}

// World is the shared game state. It is not goroutine-safe.
type World struct {
	Players      map[int]*Player
	Stars        []Star
	nextPlayerID int
	nextStarID   int
	rng          *rand.Rand
}

// NewWorld returns a world with StarCount stars and no players.
func NewWorld(seed uint64) *World {
	w := &World{
		Players:      map[int]*Player{},
		nextPlayerID: 1,
		nextStarID:   1,
		rng:          rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15)),
	}
	for i := 0; i < StarCount; i++ {
		x, z := w.spawnPoint()
		w.Stars = append(w.Stars, Star{ID: w.nextStarID, X: x, Z: z})
		w.nextStarID++
	}
	return w
}

// Resolve clamps a player centre to the arena and pushes it out of obstacles.
func Resolve(x, z float64) (float64, float64) {
	x = math.Max(-ArenaHalf, math.Min(ArenaHalf, x))
	z = math.Max(-ArenaHalf, math.Min(ArenaHalf, z))
	for _, o := range Obstacles {
		m := o.Radius + PlayerRadius
		dx, dz := x-o.X, z-o.Z
		d := math.Hypot(dx, dz)
		if d >= m {
			continue
		}
		if d < 1e-6 {
			x, z = o.X+m, o.Z
		} else {
			x, z = o.X+dx*m/d, o.Z+dz*m/d
		}
	}
	return x, z
}

// AddPlayer adds a player with the first unused animal at a spawn point.
func (w *World) AddPlayer() (*Player, error) {
	if len(w.Players) >= MaxPlayers {
		return nil, ErrFull
	}
	used := map[string]bool{}
	for _, p := range w.Players {
		used[p.Animal] = true
	}
	animal := Animals[0]
	for _, a := range Animals {
		if !used[a] {
			animal = a
			break
		}
	}
	x, z := Resolve(w.spawnPoint())
	p := &Player{ID: w.nextPlayerID, Animal: animal, X: x, Z: z}
	w.nextPlayerID++
	w.Players[p.ID] = p
	return p, nil
}

// RemovePlayer removes a player; unknown ids are ignored.
func (w *World) RemovePlayer(id int) {
	delete(w.Players, id)
}

// ApplyMove moves a player towards (x, z), limited to maxDist (no limit if
// maxDist <= 0 or +Inf), then resolves collisions. It returns false for an
// unknown id or non-finite input.
func (w *World) ApplyMove(id int, x, z, r, maxDist float64) bool {
	p, ok := w.Players[id]
	if !ok || !finite(x) || !finite(z) || !finite(r) {
		return false
	}
	if maxDist > 0 && !math.IsInf(maxDist, 1) {
		dx, dz := x-p.X, z-p.Z
		if d := math.Hypot(dx, dz); d > maxDist {
			x, z = p.X+dx*maxDist/d, p.Z+dz*maxDist/d
		}
	}
	p.X, p.Z = Resolve(x, z)
	p.R = r
	return true
}

// CollectStars awards each star within PickupRadius to the lowest-id player
// and respawns it. It returns the ids of the players who scored.
func (w *World) CollectStars() (picked []int) {
	for i, s := range w.Stars {
		var winner *Player
		for _, p := range w.Players {
			if math.Hypot(p.X-s.X, p.Z-s.Z) <= PickupRadius && (winner == nil || p.ID < winner.ID) {
				winner = p
			}
		}
		if winner == nil {
			continue
		}
		winner.Score++
		picked = append(picked, winner.ID)
		x, z := w.spawnPoint()
		w.Stars[i] = Star{ID: w.nextStarID, X: x, Z: z}
		w.nextStarID++
	}
	return picked
}

// spawnPoint picks a random free point away from obstacles, players and stars.
func (w *World) spawnPoint() (x, z float64) {
	for try := 0; try < 50; try++ {
		x = (w.rng.Float64()*2 - 1) * 5.2
		z = (w.rng.Float64()*2 - 1) * 5.2
		if w.isFree(x, z) {
			return x, z
		}
	}
	return Resolve(x, z)
}

func (w *World) isFree(x, z float64) bool {
	for _, o := range Obstacles {
		if o.Radius > 0 && math.Hypot(x-o.X, z-o.Z) < o.Radius+0.6 {
			return false
		}
	}
	for _, p := range w.Players {
		if math.Hypot(x-p.X, z-p.Z) < 1.5 {
			return false
		}
	}
	for _, s := range w.Stars {
		if math.Hypot(x-s.X, z-s.Z) < 1.5 {
			return false
		}
	}
	return true
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
