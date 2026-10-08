package gfx

import (
	"math"
	"testing"
)

// meshWithVerts returns a mesh of n vertices at the origin with +Y normals.
func meshWithVerts(n int) *Mesh {
	m := &Mesh{}
	for i := 0; i < n; i++ {
		m.Verts = append(m.Verts, 0, 0, 0, 0, 1, 0, 0, 0)
		m.Idx = append(m.Idx, uint16(i))
	}
	return m
}

func TestAppendTransforms(t *testing.T) {
	src := &Mesh{
		Verts: []float32{1, 0, 0, 1, 0, 0, 0.25, 0.75},
		Idx:   []uint16{0},
	}
	dst := meshWithVerts(2)
	dst.Append(src, Mul(Translate(0, 1, 0), Scale(2, 1, 1)))
	if dst.VertexCount() != 3 {
		t.Fatalf("VertexCount = %d, want 3", dst.VertexCount())
	}
	got := dst.Verts[16:24]
	want := []float32{2, 1, 0, 1, 0, 0, 0.25, 0.75}
	for i := range want {
		if math.Abs(float64(got[i]-want[i])) > 1e-6 {
			t.Fatalf("appended vertex = %v, want %v", got, want)
		}
	}
	if dst.Idx[2] != 2 {
		t.Errorf("appended index = %d, want 2", dst.Idx[2])
	}
}

func TestBatchChunking(t *testing.T) {
	var b Batch
	b.Add(meshWithVerts(65000), Identity())
	b.Add(meshWithVerts(535), Identity()) // exactly 65535: same chunk
	if len(b.Chunks) != 1 || b.Chunks[0].VertexCount() != MaxVerts {
		t.Fatalf("after filling: %d chunks", len(b.Chunks))
	}
	if last := b.Chunks[0].Idx[len(b.Chunks[0].Idx)-1]; last != MaxVerts-1 {
		t.Errorf("last index = %d, want %d", last, MaxVerts-1)
	}
	b.Add(meshWithVerts(1), Identity()) // 65536: new chunk
	if len(b.Chunks) != 2 || b.Chunks[1].VertexCount() != 1 || b.Chunks[1].Idx[0] != 0 {
		t.Fatalf("after overflow: %d chunks", len(b.Chunks))
	}
}
