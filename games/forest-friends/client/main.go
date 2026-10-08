//go:build js && wasm

// Command client is the Forest Friends browser client (GOOS=js GOARCH=wasm).
package main

import (
	"path"
	"syscall/js"

	"github.com/Ma-Biche/Games/games/forest-friends/game"
	"github.com/Ma-Biche/Games/games/forest-friends/gfx"
)

const (
	forestBase = "../../assets/Mini-Forest/"
	animalBase = "../../assets/Animal/"
)

// fail reports a fatal startup error and parks main forever.
func fail(msg string) {
	fatal(msg, false)
	select {}
}

func main() {
	// 1. DOM.
	doc = js.Global().Get("document")
	byID := func(id string) js.Value { return doc.Call("getElementById", id) }
	canvas = byID("game")
	loadingEl = byID("loading")
	loadErrEl = byID("load-error")
	statusEl = byID("status")
	statusIcon = byID("status-icon")
	scoreboard = byID("scoreboard")
	starsLayer = byID("stars-layer")
	padButtons = doc.Call("querySelectorAll", "#pad button[data-dir]")

	// 2. WebGL context.
	gl = canvas.Call("getContext", "webgl", map[string]any{"antialias": true})
	if gl.IsNull() {
		fail("WebGL is not supported in this browser.")
	}
	on(canvas, "webglcontextlost", func(e js.Value) {
		e.Call("preventDefault")
		fatal("Graphics context lost — reload the page", true)
	})

	// 3. Shaders.
	if !initGL() {
		select {}
	}

	// 4. Start every GLB fetch, then await them in order.
	type asset struct {
		name, url string
		req       js.Value
	}
	var forest, animals []asset
	for _, m := range game.ForestModels() {
		u := forestBase + m + ".glb"
		forest = append(forest, asset{m, u, startFetch(u)})
	}
	for _, a := range game.Animals {
		u := animalBase + "animal-" + a + ".glb"
		animals = append(animals, asset{a, u, startFetch(u)})
	}

	// 5-6. Parse, then load textures (cached by URL).
	texCache := map[string]js.Value{}
	load := func(a asset) (*gfx.Mesh, js.Value) {
		resp, err := await(a.req)
		if err != nil {
			fail("Could not load " + a.url)
		}
		data, err := readBytes(resp, a.url)
		if err != nil {
			fail(err.Error())
		}
		model, err := gfx.Parse(data)
		if err != nil {
			fail("Could not load " + a.url + ": " + err.Error())
		}
		if model.ImageURI == "" {
			return model.Mesh, whiteTexture()
		}
		texURL := path.Join(path.Dir(a.url), model.ImageURI)
		tex, ok := texCache[texURL]
		if !ok {
			img, err := loadImage(texURL)
			if err != nil {
				fail(err.Error())
			}
			tex = createTexture(img)
			texCache[texURL] = tex
		}
		return model.Mesh, tex
	}
	forestMeshes := map[string]*gfx.Mesh{}
	for i, a := range forest {
		mesh, tex := load(a)
		forestMeshes[a.name] = mesh
		if i == 0 {
			forestTex = tex
		}
	}
	animalMeshes := map[string]*gfx.Mesh{}
	for i, a := range animals {
		mesh, tex := load(a)
		animalMeshes[a.name] = mesh
		if i == 0 {
			animalTex = tex
		}
	}

	// 7. Static batch and animal meshes on the GPU.
	buildStatic(forestMeshes)
	for name, m := range animalMeshes {
		animalMesh[name] = uploadMesh(m)
	}

	// 8. Go.
	loadingEl.Get("style").Set("display", "none")
	setupInput()
	netConnect()
	frameFunc = js.FuncOf(frame)
	js.Global().Call("requestAnimationFrame", frameFunc)
	select {}
}
