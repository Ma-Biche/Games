//go:build js && wasm

package main

import (
	"syscall/js"
	"unsafe"

	"github.com/Ma-Biche/Games/games/forest-friends/gfx"
)

// WebGL 1 constants.
const (
	glDepthBufferBit     = 0x0100
	glColorBufferBit     = 0x4000
	glTriangles          = 0x0004
	glCullFace           = 0x0B44
	glDepthTest          = 0x0B71
	glTexture2D          = 0x0DE1
	glUnsignedByte       = 0x1401
	glUnsignedShort      = 0x1403
	glFloat              = 0x1406
	glRGBA               = 0x1908
	glNearest            = 0x2600
	glLinearMipmapLinear = 0x2703
	glTexMagFilter       = 0x2800
	glTexMinFilter       = 0x2801
	glTexWrapS           = 0x2802
	glTexWrapT           = 0x2803
	glClampToEdge        = 0x812F
	glTexture0           = 0x84C0
	glArrayBuffer        = 0x8892
	glElementArrayBuffer = 0x8893
	glStaticDraw         = 0x88E4
	glFragmentShader     = 0x8B30
	glVertexShader       = 0x8B31
	glCompileStatus      = 0x8B81
	glLinkStatus         = 0x8B82
)

const vertexShader = `
attribute vec3 aPos; attribute vec3 aNormal; attribute vec2 aUV;
uniform mat4 uViewProj; uniform mat4 uModel;
varying vec3 vN; varying vec2 vUV;
void main(){
  vN = (uModel * vec4(aNormal, 0.0)).xyz;
  vUV = aUV;
  gl_Position = uViewProj * uModel * vec4(aPos, 1.0);
}
`

const fragmentShader = `
precision mediump float;
uniform sampler2D uTex; varying vec3 vN; varying vec2 vUV;
void main(){
  vec3 L = normalize(vec3(0.4, 1.0, 0.3));
  float d = max(dot(normalize(vN), L), 0.0);
  vec3 c = texture2D(uTex, vUV).rgb;
  gl_FragColor = vec4(c * (0.55 + 0.5 * d), 1.0);
}
`

var (
	gl                      js.Value
	prog                    js.Value
	aPos, aNormal, aUV      int
	uViewProj, uModel, uTex js.Value
	matF32, matU8           js.Value
)

type gpuMesh struct {
	vbo, ibo js.Value
	count    int
}

func compileShader(kind int, src string) (js.Value, bool) {
	s := gl.Call("createShader", kind)
	gl.Call("shaderSource", s, src)
	gl.Call("compileShader", s)
	if !gl.Call("getShaderParameter", s, glCompileStatus).Bool() {
		consoleCall("error", gl.Call("getShaderInfoLog", s))
		return s, false
	}
	return s, true
}

// initGL compiles the program, looks up its locations and sets global state.
// On failure it calls fatal and returns false.
func initGL() bool {
	vs, ok1 := compileShader(glVertexShader, vertexShader)
	fs, ok2 := compileShader(glFragmentShader, fragmentShader)
	if !ok1 || !ok2 {
		fatal("Graphics error (shader)", false)
		return false
	}
	prog = gl.Call("createProgram")
	gl.Call("attachShader", prog, vs)
	gl.Call("attachShader", prog, fs)
	gl.Call("linkProgram", prog)
	if !gl.Call("getProgramParameter", prog, glLinkStatus).Bool() {
		consoleCall("error", gl.Call("getProgramInfoLog", prog))
		fatal("Graphics error (shader)", false)
		return false
	}

	aPos = gl.Call("getAttribLocation", prog, "aPos").Int()
	aNormal = gl.Call("getAttribLocation", prog, "aNormal").Int()
	aUV = gl.Call("getAttribLocation", prog, "aUV").Int()
	uViewProj = gl.Call("getUniformLocation", prog, "uViewProj")
	uModel = gl.Call("getUniformLocation", prog, "uModel")
	uTex = gl.Call("getUniformLocation", prog, "uTex")
	for _, a := range []int{aPos, aNormal, aUV} {
		gl.Call("enableVertexAttribArray", a)
	}

	matF32 = js.Global().Get("Float32Array").New(16)
	matU8 = js.Global().Get("Uint8Array").New(matF32.Get("buffer"))

	gl.Call("enable", glDepthTest)
	gl.Call("disable", glCullFace)
	gl.Call("clearColor", 0x0f/255.0, 0x11/255.0, 0x15/255.0, 1)
	return true
}

func jsBytes(b []byte) js.Value {
	u8 := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(u8, b)
	return u8
}

func uploadMesh(m *gfx.Mesh) gpuMesh {
	vb := unsafe.Slice((*byte)(unsafe.Pointer(&m.Verts[0])), len(m.Verts)*4)
	ib := unsafe.Slice((*byte)(unsafe.Pointer(&m.Idx[0])), len(m.Idx)*2)
	g := gpuMesh{vbo: gl.Call("createBuffer"), ibo: gl.Call("createBuffer"), count: len(m.Idx)}
	gl.Call("bindBuffer", glArrayBuffer, g.vbo)
	gl.Call("bufferData", glArrayBuffer, jsBytes(vb), glStaticDraw)
	gl.Call("bindBuffer", glElementArrayBuffer, g.ibo)
	gl.Call("bufferData", glElementArrayBuffer, jsBytes(ib), glStaticDraw)
	return g
}

func setTexParams() {
	gl.Call("generateMipmap", glTexture2D)
	gl.Call("texParameteri", glTexture2D, glTexMinFilter, glLinearMipmapLinear)
	gl.Call("texParameteri", glTexture2D, glTexMagFilter, glNearest)
	gl.Call("texParameteri", glTexture2D, glTexWrapS, glClampToEdge)
	gl.Call("texParameteri", glTexture2D, glTexWrapT, glClampToEdge)
}

// createTexture uploads a decoded image.
func createTexture(img js.Value) js.Value {
	t := gl.Call("createTexture")
	gl.Call("bindTexture", glTexture2D, t)
	gl.Call("texImage2D", glTexture2D, 0, glRGBA, glRGBA, glUnsignedByte, img)
	setTexParams()
	return t
}

// whiteTexture returns a 1×1 white texture for models without an image.
func whiteTexture() js.Value {
	t := gl.Call("createTexture")
	gl.Call("bindTexture", glTexture2D, t)
	gl.Call("texImage2D", glTexture2D, 0, glRGBA, 1, 1, 0, glRGBA, glUnsignedByte, jsBytes([]byte{255, 255, 255, 255}))
	setTexParams()
	return t
}

func setMat4(loc js.Value, m gfx.Mat4) {
	js.CopyBytesToJS(matU8, unsafe.Slice((*byte)(unsafe.Pointer(&m[0])), 64))
	gl.Call("uniformMatrix4fv", loc, false, matF32)
}

func beginFrame(viewProj gfx.Mat4) {
	gl.Call("clear", glColorBufferBit|glDepthBufferBit)
	gl.Call("useProgram", prog)
	setMat4(uViewProj, viewProj)
	gl.Call("activeTexture", glTexture0)
	gl.Call("uniform1i", uTex, 0)
}

func drawMesh(g gpuMesh, model gfx.Mat4, tex js.Value) {
	gl.Call("bindTexture", glTexture2D, tex)
	setMat4(uModel, model)
	gl.Call("bindBuffer", glArrayBuffer, g.vbo)
	gl.Call("vertexAttribPointer", aPos, 3, glFloat, false, 32, 0)
	gl.Call("vertexAttribPointer", aNormal, 3, glFloat, false, 32, 12)
	gl.Call("vertexAttribPointer", aUV, 2, glFloat, false, 32, 24)
	gl.Call("bindBuffer", glElementArrayBuffer, g.ibo)
	gl.Call("drawElements", glTriangles, g.count, glUnsignedShort, 0)
}
