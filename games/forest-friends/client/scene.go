//go:build js && wasm

package main

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/Ma-Biche/Games/games/forest-friends/game"
	"github.com/Ma-Biche/Games/games/forest-friends/gfx"
)

var (
	staticChunks         []gpuMesh
	animalMesh           = map[string]gpuMesh{}
	forestTex, animalTex js.Value

	focusX, focusZ float64
	lastFrameMs    float64
	frameFunc      js.Func
	starSprites    = map[int]js.Value{}
	scoreSig       string
)

// buildStatic bakes all scenery into a batch and uploads its chunks.
func buildStatic(meshes map[string]*gfx.Mesh) {
	var b gfx.Batch
	for _, props := range [][]game.Prop{game.Ground(), game.Fence(), game.Obstacles, game.Decor} {
		for _, p := range props {
			b.Add(meshes[p.Model], gfx.FromTRS(float32(p.X), 0, float32(p.Z), float32(p.RotYDeg*math.Pi/180), float32(p.Scale)))
		}
	}
	for _, c := range b.Chunks {
		staticChunks = append(staticChunks, uploadMesh(c))
	}
}

func resize() {
	dpr := math.Min(js.Global().Get("devicePixelRatio").Float(), 2)
	w := int(canvas.Get("clientWidth").Float() * dpr)
	h := int(canvas.Get("clientHeight").Float() * dpr)
	if canvas.Get("width").Int() != w || canvas.Get("height").Int() != h {
		canvas.Set("width", w)
		canvas.Set("height", h)
		gl.Call("viewport", 0, 0, w, h)
	}
}

func drawAnimal(animal string, x, z, r, bob float64) {
	m, ok := animalMesh[animal]
	if !ok {
		m = animalMesh["fox"]
	}
	drawMesh(m, gfx.FromTRS(float32(x), float32(game.GroundY+bob), float32(z), float32(r), game.AnimalScale), animalTex)
}

func walkBob(t float64) float64 { return 0.08 * math.Abs(math.Sin(12*t)) }

// drawStars places one pooled sprite per star; sprites of gone ids are removed.
func drawStars(vp gfx.Mat4, stars []game.StarState, t float64) {
	cssW := canvas.Get("clientWidth").Float()
	cssH := canvas.Get("clientHeight").Float()
	seen := map[int]bool{}
	for _, s := range stars {
		seen[s.ID] = true
		img, ok := starSprites[s.ID]
		if !ok {
			img = el("img", "star", "")
			img.Set("src", starIcon)
			img.Set("alt", "")
			starsLayer.Call("appendChild", img)
			starSprites[s.ID] = img
		}
		y := 0.9 + 0.1*math.Sin(2*t+float64(s.ID))
		clip := vp.MulVec4([4]float32{float32(s.X), float32(y), float32(s.Z), 1})
		style := img.Get("style")
		if clip[3] <= 0 {
			style.Set("display", "none")
			continue
		}
		px := (float64(clip[0]/clip[3])*0.5 + 0.5) * cssW
		py := (1 - (float64(clip[1]/clip[3])*0.5 + 0.5)) * cssH
		style.Set("display", "")
		style.Set("transform", fmt.Sprintf("translate(%.1fpx, %.1fpx)", px-16, py-16))
	}
	for id, img := range starSprites {
		if !seen[id] {
			img.Call("remove")
			delete(starSprites, id)
		}
	}
}

func scoreSignature(players []game.PlayerState) string {
	parts := make([]string, len(players))
	for i, p := range players {
		parts[i] = strconv.Itoa(p.ID) + ":" + strconv.Itoa(p.Score) + ":" + p.Animal
	}
	return strings.Join(parts, "|")
}

// frame is the requestAnimationFrame callback.
func frame(this js.Value, args []js.Value) any {
	if stopped {
		return nil
	}
	now := args[0].Float()
	dt := 0.0
	if lastFrameMs > 0 {
		dt = math.Max(0, math.Min((now-lastFrameMs)/1000, 0.1))
	}
	lastFrameMs = now
	t := now / 1000

	resize()

	active := mode == modeOffline || (mode == modeOnline && synced)
	moving := false
	if active {
		if dx, dz := direction(); dx != 0 || dz != 0 {
			posX, posZ = game.Resolve(posX+dx*game.MoveSpeed*dt, posZ+dz*game.MoveSpeed*dt)
			rot = math.Atan2(dx, dz)
			moving = true
		}
		if mode == modeOffline {
			world.ApplyMove(selfID, posX, posZ, rot, math.Inf(1))
			world.CollectStars()
		} else {
			maybeSend()
		}
	}

	// Camera.
	tx, tz := 0.0, 0.0
	if active {
		tx, tz = posX, posZ
	}
	k := 1 - math.Exp(-8*dt)
	focusX += (tx - focusX) * k
	focusZ += (tz - focusZ) * k
	focus := [3]float32{float32(focusX), 0, float32(focusZ)}
	eye := [3]float32{focus[0], 9, focus[2] + 7}
	view := gfx.LookAt(eye, focus, [3]float32{0, 1, 0})
	aspect := canvas.Get("width").Float() / math.Max(1, canvas.Get("height").Float())
	proj := gfx.Perspective(50*math.Pi/180, float32(aspect), 0.1, 100)
	vp := gfx.Mul(proj, view)

	beginFrame(vp)
	for _, c := range staticChunks {
		drawMesh(c, gfx.Identity(), forestTex)
	}

	var players []game.PlayerState
	var stars []game.StarState
	if active {
		bob := 0.0
		if moving {
			bob = walkBob(t)
		}
		if mode == modeOffline {
			drawAnimal(world.Players[selfID].Animal, posX, posZ, rot, bob)
			for _, s := range world.Stars {
				stars = append(stars, game.StarState{ID: s.ID, X: s.X, Z: s.Z})
			}
			for _, p := range world.Players {
				players = append(players, game.PlayerState{ID: p.ID, Animal: p.Animal, X: p.X, Z: p.Z, R: p.R, Score: p.Score})
			}
			sort.Slice(players, func(i, j int) bool { return players[i].ID < players[j].ID })
		} else {
			drawAnimal(selfAnimal, posX, posZ, rot, bob)
			ks := math.Min(1, 12*dt)
			for _, s := range remote {
				ox, oz := s.dispX, s.dispZ
				s.dispX += (s.srvX - s.dispX) * ks
				s.dispZ += (s.srvZ - s.dispZ) * ks
				rb := 0.0
				if math.Hypot(s.dispX-ox, s.dispZ-oz) > 0.01 {
					rb = walkBob(t)
				}
				drawAnimal(s.animal, s.dispX, s.dispZ, s.r, rb)
			}
			stars = onlineStars
			players = lastPlayers
		}
	}
	drawStars(vp, stars, t)

	if sig := scoreSignature(players); sig != scoreSig {
		scoreSig = sig
		renderScores(players, selfID)
	}

	js.Global().Call("requestAnimationFrame", frameFunc)
	return nil
}
