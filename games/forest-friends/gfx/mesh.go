package gfx

// MaxVerts is the most vertices a Mesh can hold with uint16 indices.
const MaxVerts = 65535

// Mesh is an interleaved triangle mesh: 8 floats per vertex (pos3, nrm3, uv2).
type Mesh struct {
	Verts []float32
	Idx   []uint16
}

// VertexCount returns the number of vertices in the mesh.
func (dst *Mesh) VertexCount() int { return len(dst.Verts) / 8 }

// Append transforms src by m (normals by NormalMatrix3(m), renormalized) and
// appends it to dst, offsetting the indices.
func (dst *Mesh) Append(src *Mesh, m Mat4) {
	nm := NormalMatrix3(m)
	base := uint16(dst.VertexCount())
	for i := 0; i+8 <= len(src.Verts); i += 8 {
		v := src.Verts[i : i+8]
		p := m.MulVec4([4]float32{v[0], v[1], v[2], 1})
		n := normalize3([3]float32{
			nm[0]*v[3] + nm[3]*v[4] + nm[6]*v[5],
			nm[1]*v[3] + nm[4]*v[4] + nm[7]*v[5],
			nm[2]*v[3] + nm[5]*v[4] + nm[8]*v[5],
		})
		dst.Verts = append(dst.Verts, p[0], p[1], p[2], n[0], n[1], n[2], v[6], v[7])
	}
	for _, ix := range src.Idx {
		dst.Idx = append(dst.Idx, base+ix)
	}
}

// Batch merges many transformed meshes into chunks of at most MaxVerts vertices.
type Batch struct {
	Chunks []*Mesh
}

// Add appends src transformed by m, starting a new chunk when the current one
// would exceed MaxVerts.
func (b *Batch) Add(src *Mesh, m Mat4) {
	n := len(b.Chunks)
	if n == 0 || b.Chunks[n-1].VertexCount()+src.VertexCount() > MaxVerts {
		b.Chunks = append(b.Chunks, &Mesh{})
		n++
	}
	b.Chunks[n-1].Append(src, m)
}
