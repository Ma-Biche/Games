package gfx

import (
	"encoding/binary"
	"encoding/json"
	"math"
)

// Model is a parsed GLB: every node transform is baked into one Mesh.
// ImageURI is the base colour texture URI, relative to the .glb file.
type Model struct {
	Mesh     *Mesh
	ImageURI string
}

type glbError string

func (e glbError) Error() string { return string(e) }

func errGLB(msg string) error { return glbError("glb: " + msg) }

type gltfDoc struct {
	Scene  *int `json:"scene"`
	Scenes []struct {
		Nodes []int `json:"nodes"`
	} `json:"scenes"`
	Nodes []struct {
		Children    []int     `json:"children"`
		Mesh        *int      `json:"mesh"`
		Matrix      []float32 `json:"matrix"`
		Translation []float32 `json:"translation"`
		Rotation    []float32 `json:"rotation"`
		Scale       []float32 `json:"scale"`
	} `json:"nodes"`
	Meshes []struct {
		Primitives []struct {
			Attributes map[string]int `json:"attributes"`
			Indices    *int           `json:"indices"`
			Mode       *int           `json:"mode"`
			Material   *int           `json:"material"`
		} `json:"primitives"`
	} `json:"meshes"`
	Accessors []struct {
		BufferView    *int            `json:"bufferView"`
		ByteOffset    int             `json:"byteOffset"`
		ComponentType int             `json:"componentType"`
		Count         int             `json:"count"`
		Type          string          `json:"type"`
		Sparse        json.RawMessage `json:"sparse"`
	} `json:"accessors"`
	BufferViews []struct {
		Buffer     int `json:"buffer"`
		ByteOffset int `json:"byteOffset"`
		ByteLength int `json:"byteLength"`
		ByteStride int `json:"byteStride"`
	} `json:"bufferViews"`
	Buffers []struct {
		URI        string `json:"uri"`
		ByteLength int    `json:"byteLength"`
	} `json:"buffers"`
	Materials []struct {
		PBR struct {
			BaseColorTexture *struct {
				Index int `json:"index"`
			} `json:"baseColorTexture"`
		} `json:"pbrMetallicRoughness"`
	} `json:"materials"`
	Textures []struct {
		Source *int `json:"source"`
	} `json:"textures"`
	Images []struct {
		URI string `json:"uri"`
	} `json:"images"`
}

const (
	chunkJSON = 0x4E4F534A
	chunkBIN  = 0x004E4942
)

// Parse decodes a GLB v2 file into one baked mesh. Errors start with "glb:".
func Parse(data []byte) (*Model, error) {
	jsonChunk, bin, err := splitChunks(data)
	if err != nil {
		return nil, err
	}
	var doc gltfDoc
	if err := json.Unmarshal(jsonChunk, &doc); err != nil {
		return nil, errGLB("bad JSON chunk: " + err.Error())
	}
	if len(doc.Buffers) != 1 || doc.Buffers[0].URI != "" {
		return nil, errGLB("only a single embedded buffer (buffers[0] without uri) is supported")
	}
	if doc.Buffers[0].ByteLength > len(bin) {
		return nil, errGLB("buffer is longer than the BIN chunk")
	}
	p := &parser{doc: &doc, bin: bin, model: &Model{Mesh: &Mesh{}}}

	scene := 0
	if doc.Scene != nil {
		scene = *doc.Scene
	}
	if scene < 0 || scene >= len(doc.Scenes) {
		return nil, errGLB("scene index out of range")
	}
	for _, n := range doc.Scenes[scene].Nodes {
		if err := p.node(n, Identity(), 0); err != nil {
			return nil, err
		}
	}
	return p.model, nil
}

// splitChunks validates the GLB header and returns the JSON and BIN chunks.
func splitChunks(data []byte) (jsonChunk, bin []byte, err error) {
	if len(data) < 20 {
		return nil, nil, errGLB("file too short")
	}
	le := binary.LittleEndian
	if string(data[0:4]) != "glTF" {
		return nil, nil, errGLB("bad magic")
	}
	if le.Uint32(data[4:8]) != 2 {
		return nil, nil, errGLB("unsupported version")
	}
	total := le.Uint32(data[8:12])
	if uint64(total) > uint64(len(data)) {
		return nil, nil, errGLB("header length exceeds file size")
	}
	data = data[:total]
	chunk := func(off int) (typ uint32, body []byte, next int, err error) {
		if off+8 > len(data) {
			return 0, nil, 0, errGLB("truncated chunk header")
		}
		n := uint64(le.Uint32(data[off : off+4]))
		if uint64(off+8)+n > uint64(len(data)) {
			return 0, nil, 0, errGLB("truncated chunk")
		}
		end := off + 8 + int(n)
		return le.Uint32(data[off+4 : off+8]), data[off+8 : end], end, nil
	}
	typ, jsonChunk, next, err := chunk(12)
	if err != nil {
		return nil, nil, err
	}
	if typ != chunkJSON {
		return nil, nil, errGLB("first chunk is not JSON")
	}
	typ, bin, _, err = chunk(next)
	if err != nil {
		return nil, nil, err
	}
	if typ != chunkBIN {
		return nil, nil, errGLB("second chunk is not BIN")
	}
	return jsonChunk, bin, nil
}

type parser struct {
	doc   *gltfDoc
	bin   []byte
	model *Model
}

// node bakes node i (with parent world matrix) and its children.
func (p *parser) node(i int, parent Mat4, depth int) error {
	if i < 0 || i >= len(p.doc.Nodes) {
		return errGLB("node index out of range")
	}
	if depth > len(p.doc.Nodes) {
		return errGLB("node hierarchy has a cycle")
	}
	n := p.doc.Nodes[i]
	var local Mat4
	switch {
	case len(n.Matrix) == 16:
		copy(local[:], n.Matrix)
	case len(n.Matrix) != 0:
		return errGLB("node matrix must have 16 values")
	default:
		t, q, s := []float32{0, 0, 0}, []float32{0, 0, 0, 1}, []float32{1, 1, 1}
		if n.Translation != nil {
			t = n.Translation
		}
		if n.Rotation != nil {
			q = n.Rotation
		}
		if n.Scale != nil {
			s = n.Scale
		}
		if len(t) != 3 || len(q) != 4 || len(s) != 3 {
			return errGLB("bad node translation/rotation/scale")
		}
		local = Mul(Mul(Translate(t[0], t[1], t[2]), FromQuat(q[0], q[1], q[2], q[3])), Scale(s[0], s[1], s[2]))
	}
	world := Mul(parent, local)
	if n.Mesh != nil {
		if err := p.mesh(*n.Mesh, world); err != nil {
			return err
		}
	}
	for _, c := range n.Children {
		if err := p.node(c, world, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// mesh bakes every primitive of mesh i with the given world matrix.
func (p *parser) mesh(i int, world Mat4) error {
	if i < 0 || i >= len(p.doc.Meshes) {
		return errGLB("mesh index out of range")
	}
	for _, prim := range p.doc.Meshes[i].Primitives {
		if prim.Mode != nil && *prim.Mode != 4 {
			return errGLB("only triangle primitives (mode 4) are supported")
		}
		pos, err := p.floats(prim.Attributes, "POSITION", "VEC3", 3)
		if err != nil {
			return err
		}
		nrm, err := p.floats(prim.Attributes, "NORMAL", "VEC3", 3)
		if err != nil {
			return err
		}
		uv, err := p.floats(prim.Attributes, "TEXCOORD_0", "VEC2", 2)
		if err != nil {
			return err
		}
		n := len(pos) / 3
		if len(nrm)/3 != n || len(uv)/2 != n {
			return errGLB("attribute counts differ")
		}
		if p.model.Mesh.VertexCount()+n > MaxVerts {
			return errGLB("model has more than 65535 vertices")
		}
		src := &Mesh{Verts: make([]float32, 0, n*8)}
		for v := 0; v < n; v++ {
			src.Verts = append(src.Verts,
				pos[v*3], pos[v*3+1], pos[v*3+2],
				nrm[v*3], nrm[v*3+1], nrm[v*3+2],
				uv[v*2], uv[v*2+1])
		}
		if prim.Indices == nil {
			for v := 0; v < n; v++ {
				src.Idx = append(src.Idx, uint16(v))
			}
		} else if src.Idx, err = p.indices(*prim.Indices, n); err != nil {
			return err
		}
		p.model.Mesh.Append(src, world)
		if p.model.ImageURI == "" && prim.Material != nil {
			if p.model.ImageURI, err = p.imageURI(*prim.Material); err != nil {
				return err
			}
		}
	}
	return nil
}

// view returns accessor i, its bytes (starting at the accessor's first
// element) and its stride, after checking every element fits.
func (p *parser) view(i int, elemSize int) (count int, data []byte, stride int, ctype int, typ string, err error) {
	if i < 0 || i >= len(p.doc.Accessors) {
		return 0, nil, 0, 0, "", errGLB("accessor index out of range")
	}
	a := p.doc.Accessors[i]
	if len(a.Sparse) != 0 && string(a.Sparse) != "null" {
		return 0, nil, 0, 0, "", errGLB("sparse accessors are not supported")
	}
	if a.BufferView == nil || *a.BufferView < 0 || *a.BufferView >= len(p.doc.BufferViews) {
		return 0, nil, 0, 0, "", errGLB("accessor has no valid bufferView")
	}
	bv := p.doc.BufferViews[*a.BufferView]
	if bv.Buffer != 0 || bv.ByteOffset < 0 || bv.ByteLength < 0 ||
		bv.ByteOffset > len(p.bin) || bv.ByteLength > len(p.bin)-bv.ByteOffset {
		return 0, nil, 0, 0, "", errGLB("bufferView out of range")
	}
	data = p.bin[bv.ByteOffset : bv.ByteOffset+bv.ByteLength]
	stride = bv.ByteStride
	if stride == 0 {
		stride = elemSize
	}
	// The size limits keep the bounds arithmetic below from overflowing.
	if a.Count < 0 || a.ByteOffset < 0 || stride < elemSize ||
		a.Count > len(data) || stride > len(data)+elemSize || a.ByteOffset > len(data) {
		return 0, nil, 0, 0, "", errGLB("accessor out of bounds")
	}
	if a.Count > 0 && a.ByteOffset+(a.Count-1)*stride+elemSize > len(data) {
		return 0, nil, 0, 0, "", errGLB("accessor out of bounds")
	}
	return a.Count, data[a.ByteOffset:], stride, a.ComponentType, a.Type, nil
}

// floats reads a float attribute with comps components per element.
func (p *parser) floats(attrs map[string]int, name, typ string, comps int) ([]float32, error) {
	i, ok := attrs[name]
	if !ok {
		return nil, errGLB("primitive has no " + name)
	}
	count, data, stride, ctype, atype, err := p.view(i, comps*4)
	if err != nil {
		return nil, err
	}
	if ctype != 5126 || atype != typ {
		return nil, errGLB(name + " must be float " + typ)
	}
	out := make([]float32, 0, count*comps)
	for e := 0; e < count; e++ {
		for c := 0; c < comps; c++ {
			off := e*stride + c*4
			out = append(out, math.Float32frombits(binary.LittleEndian.Uint32(data[off:off+4])))
		}
	}
	return out, nil
}

// indices reads index accessor i as uint16, checking each is < n.
func (p *parser) indices(i, n int) ([]uint16, error) {
	if i < 0 || i >= len(p.doc.Accessors) {
		return nil, errGLB("accessor index out of range")
	}
	size := map[int]int{5121: 1, 5123: 2, 5125: 4}[p.doc.Accessors[i].ComponentType]
	if size == 0 {
		return nil, errGLB("unsupported index component type")
	}
	count, data, stride, _, typ, err := p.view(i, size)
	if err != nil {
		return nil, err
	}
	if typ != "SCALAR" {
		return nil, errGLB("indices must be SCALAR")
	}
	out := make([]uint16, count)
	for e := range out {
		off := e * stride
		var v uint32
		switch size {
		case 1:
			v = uint32(data[off])
		case 2:
			v = uint32(binary.LittleEndian.Uint16(data[off : off+2]))
		default:
			v = binary.LittleEndian.Uint32(data[off : off+4])
		}
		if v >= uint32(n) {
			return nil, errGLB("index out of range")
		}
		out[e] = uint16(v)
	}
	return out, nil
}

// imageURI follows material → baseColorTexture → texture → image.
func (p *parser) imageURI(mat int) (string, error) {
	if mat < 0 || mat >= len(p.doc.Materials) {
		return "", errGLB("material index out of range")
	}
	bct := p.doc.Materials[mat].PBR.BaseColorTexture
	if bct == nil {
		return "", nil
	}
	if bct.Index < 0 || bct.Index >= len(p.doc.Textures) {
		return "", errGLB("texture index out of range")
	}
	src := p.doc.Textures[bct.Index].Source
	if src == nil {
		return "", nil
	}
	if *src < 0 || *src >= len(p.doc.Images) {
		return "", errGLB("image index out of range")
	}
	return p.doc.Images[*src].URI, nil
}
