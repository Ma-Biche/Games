package game

import (
	"errors"
	"math"
	"testing"
)

// newTestWorld returns a world with one player placed at (x, z).
func newTestWorld(t *testing.T, x, z float64) (*World, *Player) {
	t.Helper()
	w := NewWorld(1)
	p, err := w.AddPlayer()
	if err != nil {
		t.Fatal(err)
	}
	p.X, p.Z = x, z
	return w, p
}

func TestApplyMoveClampsStep(t *testing.T) {
	w, p := newTestWorld(t, 0, 0)
	if !w.ApplyMove(p.ID, 0, 3, 1, 0.5) {
		t.Fatal("ApplyMove returned false")
	}
	if math.Abs(p.X) > 1e-9 || math.Abs(p.Z-0.5) > 1e-9 || p.R != 1 {
		t.Errorf("player at (%v,%v) r=%v, want (0,0.5) r=1", p.X, p.Z, p.R)
	}
}

func TestApplyMoveRejectsNonFinite(t *testing.T) {
	w, p := newTestWorld(t, 0, 0)
	bad := []float64{math.NaN(), math.Inf(1), math.Inf(-1)}
	for _, v := range bad {
		for _, args := range [][3]float64{{v, 0, 0}, {0, v, 0}, {0, 0, v}} {
			if w.ApplyMove(p.ID, args[0], args[1], args[2], 1) {
				t.Errorf("ApplyMove%v accepted", args)
			}
		}
	}
	if p.X != 0 || p.Z != 0 || p.R != 0 {
		t.Errorf("player moved to (%v,%v) r=%v", p.X, p.Z, p.R)
	}
}

func TestApplyMoveUnknownID(t *testing.T) {
	w, _ := newTestWorld(t, 0, 0)
	if w.ApplyMove(99, 0, 0, 0, 1) {
		t.Error("ApplyMove accepted an unknown id")
	}
}

func TestApplyMoveInfMaxDist(t *testing.T) {
	w, p := newTestWorld(t, 0, 0)
	if !w.ApplyMove(p.ID, 3, 0, 0, math.Inf(1)) {
		t.Fatal("ApplyMove returned false")
	}
	if x, z := Resolve(3, 0); p.X != x || p.Z != z {
		t.Errorf("player at (%v,%v), want (%v,%v)", p.X, p.Z, x, z)
	}
}

func TestApplyMoveZeroMaxDistNoClamp(t *testing.T) {
	w, p := newTestWorld(t, 0, 0)
	if !w.ApplyMove(p.ID, 0, 3, 0, 0) {
		t.Fatal("ApplyMove returned false")
	}
	if x, z := Resolve(0, 3); p.X != x || p.Z != z {
		t.Errorf("player at (%v,%v), want (%v,%v)", p.X, p.Z, x, z)
	}
}

func TestCollectStarsScores(t *testing.T) {
	w, p := newTestWorld(t, 0, 0)
	old := w.Stars[2]
	p.X, p.Z = old.X+0.3, old.Z
	picked := w.CollectStars()
	if len(picked) != 1 || picked[0] != p.ID {
		t.Fatalf("picked = %v, want [%d]", picked, p.ID)
	}
	if p.Score != 1 {
		t.Errorf("score = %d, want 1", p.Score)
	}
	if len(w.Stars) != StarCount {
		t.Errorf("star count = %d, want %d", len(w.Stars), StarCount)
	}
	if w.Stars[2].ID <= old.ID {
		t.Errorf("new star id %d not greater than old %d", w.Stars[2].ID, old.ID)
	}
	for i, s := range w.Stars {
		if s.ID == old.ID {
			t.Errorf("old star still present at %d", i)
		}
	}
}

func TestCollectStarsLowestIDWins(t *testing.T) {
	w, p1 := newTestWorld(t, 0, 0)
	p2, err := w.AddPlayer()
	if err != nil {
		t.Fatal(err)
	}
	s := w.Stars[0]
	p1.X, p1.Z = s.X+0.2, s.Z
	p2.X, p2.Z = s.X-0.2, s.Z
	picked := w.CollectStars()
	if len(picked) != 1 || picked[0] != p1.ID {
		t.Fatalf("picked = %v, want [%d]", picked, p1.ID)
	}
	if p1.Score != 1 || p2.Score != 0 {
		t.Errorf("scores = %d, %d, want 1, 0", p1.Score, p2.Score)
	}
}

func TestAddPlayer(t *testing.T) {
	w := NewWorld(7)
	seen := map[string]bool{}
	for i := 1; i <= MaxPlayers; i++ {
		p, err := w.AddPlayer()
		if err != nil {
			t.Fatalf("player %d: %v", i, err)
		}
		if p.ID != i {
			t.Errorf("id = %d, want %d", p.ID, i)
		}
		if seen[p.Animal] {
			t.Errorf("animal %q reused", p.Animal)
		}
		seen[p.Animal] = true
	}
	if _, err := w.AddPlayer(); !errors.Is(err, ErrFull) {
		t.Fatalf("9th AddPlayer err = %v, want ErrFull", err)
	}
	w.RemovePlayer(3)
	w.RemovePlayer(3)
	p, err := w.AddPlayer()
	if err != nil {
		t.Fatal(err)
	}
	if p.Animal != Animals[2] || p.ID != MaxPlayers+1 {
		t.Errorf("got id %d animal %q, want id %d animal %q", p.ID, p.Animal, MaxPlayers+1, Animals[2])
	}
}

func TestSpawnPointConstraints(t *testing.T) {
	w := NewWorld(42)
	if _, err := w.AddPlayer(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		x, z := w.spawnPoint()
		if math.Abs(x) > 5.2 || math.Abs(z) > 5.2 {
			t.Errorf("spawn (%v,%v) outside ±5.2", x, z)
		}
		for _, o := range Obstacles {
			if math.Hypot(x-o.X, z-o.Z) < o.Radius+0.6 {
				t.Errorf("spawn (%v,%v) too close to %s", x, z, o.Model)
			}
		}
		for _, p := range w.Players {
			if math.Hypot(x-p.X, z-p.Z) < 1.5 {
				t.Errorf("spawn (%v,%v) too close to player %d", x, z, p.ID)
			}
		}
		for _, s := range w.Stars {
			if math.Hypot(x-s.X, z-s.Z) < 1.5 {
				t.Errorf("spawn (%v,%v) too close to star %d", x, z, s.ID)
			}
		}
	}
}
