package game

import (
	"math"
	"testing"
)

func inflated(o Prop) float64 { return o.Radius + PlayerRadius }

func TestObstaclesInsideArena(t *testing.T) {
	for _, o := range Obstacles {
		m := inflated(o)
		if math.Abs(o.X)+m > ArenaHalf || math.Abs(o.Z)+m > ArenaHalf {
			t.Errorf("%s at (%v,%v) reaches outside the arena", o.Model, o.X, o.Z)
		}
	}
}

func TestObstaclesDoNotOverlap(t *testing.T) {
	for i, a := range Obstacles {
		for _, b := range Obstacles[i+1:] {
			if d := math.Hypot(a.X-b.X, a.Z-b.Z); d < inflated(a)+inflated(b) {
				t.Errorf("%s (%v,%v) and %s (%v,%v) overlap: d=%v", a.Model, a.X, a.Z, b.Model, b.X, b.Z, d)
			}
		}
	}
}

func TestGround(t *testing.T) {
	g := Ground()
	if len(g) != 144 {
		t.Fatalf("len(Ground()) = %d, want 144", len(g))
	}
	for _, p := range g {
		if p.Model != "patch-grass" || p.Scale != 1 || math.Abs(p.X) > 5.5 || math.Abs(p.Z) > 5.5 {
			t.Errorf("bad ground tile %+v", p)
		}
	}
}

func TestFence(t *testing.T) {
	f := Fence()
	if len(f) != 52 {
		t.Fatalf("len(Fence()) = %d, want 52", len(f))
	}
	fences := 0
	caps := map[[2]float64]bool{}
	for _, p := range f {
		switch p.Model {
		case "fence":
			fences++
			if p.Radius != 0 || p.Scale != 1 {
				t.Errorf("bad fence %+v", p)
			}
		case "stones":
			if p.Scale != 0.8 || p.Radius != 0 || p.RotYDeg != 0 {
				t.Errorf("bad corner cap %+v", p)
			}
			caps[[2]float64{p.X, p.Z}] = true
		default:
			t.Errorf("unexpected model %q", p.Model)
		}
	}
	if fences != 48 {
		t.Errorf("fence pieces = %d, want 48", fences)
	}
	for _, x := range []float64{-6.1, 6.1} {
		for _, z := range []float64{-6.1, 6.1} {
			if !caps[[2]float64{x, z}] {
				t.Errorf("missing corner cap at (%v,%v)", x, z)
			}
		}
	}
}

func TestResolveGrid(t *testing.T) {
	for i := -70; i <= 70; i++ {
		for j := -70; j <= 70; j++ {
			x, z := Resolve(float64(i)/10, float64(j)/10)
			if math.Abs(x) > ArenaHalf+1e-9 || math.Abs(z) > ArenaHalf+1e-9 {
				t.Fatalf("Resolve(%v,%v) = (%v,%v) out of bounds", float64(i)/10, float64(j)/10, x, z)
			}
			for _, o := range Obstacles {
				if d := math.Hypot(x-o.X, z-o.Z); d < inflated(o)-1e-9 {
					t.Fatalf("Resolve(%v,%v) = (%v,%v) inside %s (d=%v)", float64(i)/10, float64(j)/10, x, z, o.Model, d)
				}
			}
		}
	}
}

func TestResolveDegenerateCentre(t *testing.T) {
	o := Obstacles[0]
	x, z := Resolve(o.X, o.Z)
	if wantX := o.X + inflated(o); math.Abs(x-wantX) > 1e-9 || math.Abs(z-o.Z) > 1e-9 {
		t.Errorf("Resolve(centre) = (%v,%v), want (%v,%v)", x, z, wantX, o.Z)
	}
}
