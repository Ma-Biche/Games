package gfx

import (
	"math"
	"testing"
)

const tol = 1e-5

func near(a, b float32) bool { return math.Abs(float64(a-b)) <= tol }

func assertVec4(t *testing.T, name string, got, want [4]float32) {
	t.Helper()
	for i := range got {
		if !near(got[i], want[i]) {
			t.Errorf("%s = %v, want %v", name, got, want)
			return
		}
	}
}

func assertMat4(t *testing.T, name string, got, want Mat4) {
	t.Helper()
	for i := range got {
		if !near(got[i], want[i]) {
			t.Errorf("%s = %v, want %v", name, got, want)
			return
		}
	}
}

func TestMulIdentity(t *testing.T) {
	m := FromTRS(1, 2, 3, 0.7, 1.5)
	assertMat4(t, "Mul(Identity(), m)", Mul(Identity(), m), m)
	assertMat4(t, "Mul(m, Identity())", Mul(m, Identity()), m)
}

func TestRotateYSign(t *testing.T) {
	z := [4]float32{0, 0, 1, 0}
	assertVec4(t, "RotateY(π/2)·Z", RotateY(math.Pi/2).MulVec4(z), [4]float32{1, 0, 0, 0})
	assertVec4(t, "RotateY(π)·Z", RotateY(math.Pi).MulVec4(z), [4]float32{0, 0, -1, 0})
	assertVec4(t, "RotateY(π/2)·X", RotateY(math.Pi/2).MulVec4([4]float32{1, 0, 0, 0}), [4]float32{0, 0, -1, 0})
}

func TestFromTRS(t *testing.T) {
	got := FromTRS(1, 2, 3, math.Pi/2, 2).MulVec4([4]float32{0, 0, 1, 1})
	assertVec4(t, "FromTRS·(0,0,1,1)", got, [4]float32{3, 2, 3, 1})
}

func TestFromQuatMatchesRotateY(t *testing.T) {
	s, c := float32(math.Sin(math.Pi/4)), float32(math.Cos(math.Pi/4))
	assertMat4(t, "FromQuat", FromQuat(0, s, 0, c), RotateY(math.Pi/2))
}

func TestLookAt(t *testing.T) {
	v := LookAt([3]float32{0, 9, 7}, [3]float32{0, 0, 0}, [3]float32{0, 1, 0})
	assertVec4(t, "view·eye", v.MulVec4([4]float32{0, 9, 7, 1}), [4]float32{0, 0, 0, 1})
	assertVec4(t, "view·target", v.MulVec4([4]float32{0, 0, 0, 1}), [4]float32{0, 0, -float32(math.Sqrt(130)), 1})
}

func TestPerspectiveDepth(t *testing.T) {
	p := Perspective(math.Pi/2, 1, 0.1, 100)
	for _, c := range []struct{ z, ndc float32 }{{-0.1, -1}, {-100, 1}} {
		v := p.MulVec4([4]float32{0, 0, c.z, 1})
		if got := v[2] / v[3]; !near(got, c.ndc) {
			t.Errorf("NDC z for view z=%v = %v, want %v", c.z, got, c.ndc)
		}
	}
}

func TestNormalMatrix3(t *testing.T) {
	n := NormalMatrix3(Scale(2, 1, 1))
	if !near(n[0], 0.5) || !near(n[4], 1) || !near(n[8], 1) {
		t.Errorf("NormalMatrix3(Scale(2,1,1)) = %v", n)
	}
	sing := Scale(1, 0, 3)
	sing[4] = 5 // non-diagonal element to check the copy layout
	want := [9]float32{1, 0, 0, 5, 0, 0, 0, 0, 3}
	if got := NormalMatrix3(sing); got != want {
		t.Errorf("NormalMatrix3(singular) = %v, want %v", got, want)
	}
}
