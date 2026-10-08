// Package gfx holds the host-testable graphics helpers: matrices, GLB
// loading and mesh batching.
package gfx

import "math"

// Mat4 is column-major: element (row r, col c) is at m[c*4+r], the same
// layout as glTF `matrix` and uniformMatrix4fv(transpose=false).
type Mat4 [16]float32

// Identity returns the identity matrix.
func Identity() Mat4 {
	return Mat4{0: 1, 5: 1, 10: 1, 15: 1}
}

// Mul returns a·b (b is applied to a vector first).
func Mul(a, b Mat4) Mat4 {
	var m Mat4
	for c := 0; c < 4; c++ {
		for r := 0; r < 4; r++ {
			var s float32
			for k := 0; k < 4; k++ {
				s += a[k*4+r] * b[c*4+k]
			}
			m[c*4+r] = s
		}
	}
	return m
}

// Translate returns a translation matrix.
func Translate(x, y, z float32) Mat4 {
	m := Identity()
	m[12], m[13], m[14] = x, y, z
	return m
}

// RotateY rotates around +Y (right-handed): +Z maps to (sin rad, 0, cos rad)
// and +X maps to (cos rad, 0, −sin rad).
func RotateY(rad float32) Mat4 {
	s := float32(math.Sin(float64(rad)))
	c := float32(math.Cos(float64(rad)))
	m := Identity()
	m[0], m[2] = c, -s
	m[8], m[10] = s, c
	return m
}

// Scale returns a scale matrix.
func Scale(x, y, z float32) Mat4 {
	return Mat4{0: x, 5: y, 10: z, 15: 1}
}

// FromQuat returns the rotation for a unit quaternion in glTF order (x, y, z, w).
func FromQuat(x, y, z, w float32) Mat4 {
	return Mat4{
		1 - 2*(y*y+z*z), 2 * (x*y + z*w), 2 * (x*z - y*w), 0,
		2 * (x*y - z*w), 1 - 2*(x*x+z*z), 2 * (y*z + x*w), 0,
		2 * (x*z + y*w), 2 * (y*z - x*w), 1 - 2*(x*x+y*y), 0,
		0, 0, 0, 1,
	}
}

// FromTRS returns Translate·RotateY·Scale (uniform scale). Used for runtime
// and prop placement only.
func FromTRS(tx, ty, tz, rotYRad, s float32) Mat4 {
	return Mul(Mul(Translate(tx, ty, tz), RotateY(rotYRad)), Scale(s, s, s))
}

// Perspective returns an OpenGL projection (clip z in [−w, w], camera looks down −Z).
func Perspective(fovYRad, aspect, near, far float32) Mat4 {
	f := float32(1 / math.Tan(float64(fovYRad)/2))
	return Mat4{
		0:  f / aspect,
		5:  f,
		10: (far + near) / (near - far),
		11: -1,
		14: 2 * far * near / (near - far),
	}
}

// LookAt returns a right-handed view matrix (gluLookAt).
func LookAt(eye, center, up [3]float32) Mat4 {
	f := normalize3(sub3(center, eye))
	s := normalize3(cross3(f, up))
	u := cross3(s, f)
	return Mat4{
		s[0], u[0], -f[0], 0,
		s[1], u[1], -f[1], 0,
		s[2], u[2], -f[2], 0,
		-dot3(s, eye), -dot3(u, eye), dot3(f, eye), 1,
	}
}

// MulVec4 returns m·v.
func (m Mat4) MulVec4(v [4]float32) [4]float32 {
	var out [4]float32
	for r := 0; r < 4; r++ {
		out[r] = m[r]*v[0] + m[4+r]*v[1] + m[8+r]*v[2] + m[12+r]*v[3]
	}
	return out
}

// NormalMatrix3 returns the inverse-transpose of the upper 3×3 of m,
// column-major. If |det| < 1e-12 the upper 3×3 is returned unchanged.
func NormalMatrix3(m Mat4) [9]float32 {
	var a [3][3]float64 // a[r][c]
	for c := 0; c < 3; c++ {
		for r := 0; r < 3; r++ {
			a[r][c] = float64(m[c*4+r])
		}
	}
	var cof [3][3]float64
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			r1, r2 := (r+1)%3, (r+2)%3
			c1, c2 := (c+1)%3, (c+2)%3
			cof[r][c] = a[r1][c1]*a[r2][c2] - a[r1][c2]*a[r2][c1]
		}
	}
	det := a[0][0]*cof[0][0] + a[0][1]*cof[0][1] + a[0][2]*cof[0][2]
	var out [9]float32
	for c := 0; c < 3; c++ {
		for r := 0; r < 3; r++ {
			if math.Abs(det) < 1e-12 {
				out[c*3+r] = float32(a[r][c])
			} else {
				out[c*3+r] = float32(cof[r][c] / det)
			}
		}
	}
	return out
}

func sub3(a, b [3]float32) [3]float32 { return [3]float32{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }

func dot3(a, b [3]float32) float32 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }

func cross3(a, b [3]float32) [3]float32 {
	return [3]float32{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}

func normalize3(v [3]float32) [3]float32 {
	l := float32(math.Sqrt(float64(dot3(v, v))))
	if l == 0 {
		return v
	}
	return [3]float32{v[0] / l, v[1] / l, v[2] / l}
}
