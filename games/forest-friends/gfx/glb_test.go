package gfx

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
)

func parseFile(t *testing.T, path string) *Model {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse(%s): %v", path, err)
	}
	return m
}

func TestParseFox(t *testing.T) {
	m := parseFile(t, "../../../assets/Animal/animal-fox.glb")
	if n := m.Mesh.VertexCount(); n != 993 {
		t.Errorf("fox vertices = %d, want 993", n)
	}
	if m.ImageURI != "Textures/colormap.png" {
		t.Errorf("ImageURI = %q", m.ImageURI)
	}
	for i := 0; i < len(m.Mesh.Verts); i += 8 {
		if y := m.Mesh.Verts[i+1]; y < -0.01 {
			t.Fatalf("fox vertex %d has y = %v", i/8, y)
		}
	}
	for _, ix := range m.Mesh.Idx {
		if int(ix) >= m.Mesh.VertexCount() {
			t.Fatalf("index %d out of range", ix)
		}
	}
}

func TestParseCowNormals(t *testing.T) {
	m := parseFile(t, "../../../assets/Animal/animal-cow.glb")
	for i := 0; i < len(m.Mesh.Verts); i += 8 {
		v := m.Mesh.Verts[i+3 : i+6]
		l := math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2]))
		if math.Abs(l-1) > 1e-4 {
			t.Fatalf("cow normal %d has length %v", i/8, l)
		}
	}
}

func TestParsePatchGrass(t *testing.T) {
	m := parseFile(t, "../../../assets/Mini-Forest/patch-grass.glb")
	maxY := float32(math.Inf(-1))
	flat := 0
	for i := 0; i < len(m.Mesh.Verts); i += 8 {
		y := m.Mesh.Verts[i+1]
		if y > maxY {
			maxY = y
		}
		if math.Abs(float64(y)-0.05) < 1e-3 {
			flat++
		}
	}
	if math.Abs(float64(maxY)-0.171) > 1e-3 {
		t.Errorf("patch-grass max y = %v, want ≈ 0.171", maxY)
	}
	if flat < 60 {
		t.Errorf("flat vertices at y=0.05: %d, want >= 60", flat)
	}
}

// triangleDoc returns a one-triangle glTF document and its BIN data, with
// u8 indices [2,1,0] and a node translated by (0,2,0).
func triangleDoc() (map[string]any, []byte) {
	var bin bytes.Buffer
	floats := []float32{
		0, 0, 0, 1, 0, 0, 0, 0, 1, // POSITION
		0, 1, 0, 0, 1, 0, 0, 1, 0, // NORMAL
		0, 0, 1, 0, 0, 1, // TEXCOORD_0
	}
	binary.Write(&bin, binary.LittleEndian, floats)
	bin.Write([]byte{2, 1, 0})
	doc := map[string]any{
		"asset":  map[string]any{"version": "2.0"},
		"scene":  0,
		"scenes": []any{map[string]any{"nodes": []int{0}}},
		"nodes":  []any{map[string]any{"mesh": 0, "translation": []float32{0, 2, 0}}},
		"meshes": []any{map[string]any{"primitives": []any{map[string]any{
			"attributes": map[string]int{"POSITION": 0, "NORMAL": 1, "TEXCOORD_0": 2},
			"indices":    3,
			"material":   0,
		}}}},
		"accessors": []any{
			map[string]any{"bufferView": 0, "componentType": 5126, "count": 3, "type": "VEC3"},
			map[string]any{"bufferView": 1, "componentType": 5126, "count": 3, "type": "VEC3"},
			map[string]any{"bufferView": 2, "componentType": 5126, "count": 3, "type": "VEC2"},
			map[string]any{"bufferView": 3, "componentType": 5121, "count": 3, "type": "SCALAR"},
		},
		"bufferViews": []any{
			map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": 36},
			map[string]any{"buffer": 0, "byteOffset": 36, "byteLength": 36},
			map[string]any{"buffer": 0, "byteOffset": 72, "byteLength": 24},
			map[string]any{"buffer": 0, "byteOffset": 96, "byteLength": 3},
		},
		"buffers":   []any{map[string]any{"byteLength": bin.Len()}},
		"materials": []any{map[string]any{"pbrMetallicRoughness": map[string]any{"baseColorTexture": map[string]any{"index": 0}}}},
		"textures":  []any{map[string]any{"source": 0}},
		"images":    []any{map[string]any{"uri": "Textures/colormap.png"}},
	}
	return doc, bin.Bytes()
}

// buildGLB packs a glTF document and BIN data into a GLB v2 file.
func buildGLB(t *testing.T, doc map[string]any, bin []byte) []byte {
	t.Helper()
	js, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	for len(js)%4 != 0 {
		js = append(js, ' ')
	}
	bin = append([]byte(nil), bin...)
	for len(bin)%4 != 0 {
		bin = append(bin, 0)
	}
	le := binary.LittleEndian
	var out []byte
	out = le.AppendUint32(append(out, "glTF"...), 2)
	out = le.AppendUint32(out, uint32(12+8+len(js)+8+len(bin)))
	out = le.AppendUint32(le.AppendUint32(out, uint32(len(js))), chunkJSON)
	out = append(out, js...)
	out = le.AppendUint32(le.AppendUint32(out, uint32(len(bin))), chunkBIN)
	return append(out, bin...)
}

func TestParseSyntheticU8Indices(t *testing.T) {
	doc, bin := triangleDoc()
	m, err := Parse(buildGLB(t, doc, bin))
	if err != nil {
		t.Fatal(err)
	}
	if m.Mesh.VertexCount() != 3 {
		t.Fatalf("vertices = %d, want 3", m.Mesh.VertexCount())
	}
	if got := m.Mesh.Idx; len(got) != 3 || got[0] != 2 || got[1] != 1 || got[2] != 0 {
		t.Errorf("indices = %v, want [2 1 0]", got)
	}
	if v := m.Mesh.Verts[8:16]; v[0] != 1 || v[1] != 2 || v[2] != 0 || v[4] != 1 || v[6] != 1 || v[7] != 0 {
		t.Errorf("vertex 1 = %v, want pos (1,2,0) normal +Y uv (1,0)", v)
	}
	if m.ImageURI != "Textures/colormap.png" {
		t.Errorf("ImageURI = %q", m.ImageURI)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]func(t *testing.T) []byte{
		"bad magic": func(t *testing.T) []byte {
			doc, bin := triangleDoc()
			data := buildGLB(t, doc, bin)
			copy(data, "glTX")
			return data
		},
		"truncated chunk": func(t *testing.T) []byte {
			doc, bin := triangleDoc()
			data := buildGLB(t, doc, bin)
			binary.LittleEndian.PutUint32(data[12:], uint32(len(data)))
			return data
		},
		"uri buffer": func(t *testing.T) []byte {
			doc, bin := triangleDoc()
			doc["buffers"] = []any{map[string]any{"uri": "data.bin", "byteLength": len(bin)}}
			return buildGLB(t, doc, bin)
		},
		"missing POSITION": func(t *testing.T) []byte {
			doc, bin := triangleDoc()
			prim := doc["meshes"].([]any)[0].(map[string]any)["primitives"].([]any)[0].(map[string]any)
			delete(prim["attributes"].(map[string]int), "POSITION")
			return buildGLB(t, doc, bin)
		},
	}
	want := map[string]string{
		"bad magic":        "glb: bad magic",
		"truncated chunk":  "glb: truncated chunk",
		"uri buffer":       "glb: only a single embedded buffer",
		"missing POSITION": "glb: primitive has no POSITION",
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(build(t))
			if err == nil {
				t.Fatal("Parse succeeded, want an error")
			}
			if !strings.HasPrefix(err.Error(), want[name]) {
				t.Errorf("error %q, want prefix %q", err, want[name])
			}
		})
	}
}
