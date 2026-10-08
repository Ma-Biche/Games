//go:build js && wasm

package main

import (
	"math"
	"strings"
	"syscall/js"
)

var keyDirs = map[string]string{
	"KeyW": "up", "ArrowUp": "up",
	"KeyS": "down", "ArrowDown": "down",
	"KeyA": "left", "ArrowLeft": "left",
	"KeyD": "right", "ArrowRight": "right",
}

// held holds the pressed directions ("up", "down", "left", "right").
var held = map[string]bool{}

func on(target js.Value, event string, fn func(e js.Value)) {
	target.Call("addEventListener", event, js.FuncOf(func(this js.Value, args []js.Value) any {
		fn(args[0])
		return nil
	}))
}

func keyHandler(down bool) func(e js.Value) {
	return func(e js.Value) {
		code := e.Get("code").String()
		dir, ok := keyDirs[code]
		if !ok {
			return
		}
		if strings.HasPrefix(code, "Arrow") {
			e.Call("preventDefault")
		}
		held[dir] = down
	}
}

func setupInput() {
	win := js.Global()
	on(win, "keydown", keyHandler(true))
	on(win, "keyup", keyHandler(false))
	on(win, "blur", func(js.Value) { clear(held) })

	for i := 0; i < padButtons.Get("length").Int(); i++ {
		btn := padButtons.Index(i)
		dir := btn.Get("dataset").Get("dir").String()
		on(btn, "pointerdown", func(e js.Value) {
			e.Call("preventDefault")
			e.Get("currentTarget").Call("releasePointerCapture", e.Get("pointerId"))
			held[dir] = true
		})
		for _, ev := range []string{"pointerup", "pointercancel", "pointerleave"} {
			on(btn, ev, func(js.Value) { held[dir] = false })
		}
	}
	on(doc.Call("getElementById", "pad"), "contextmenu", func(e js.Value) { e.Call("preventDefault") })
}

// direction returns the normalized input direction: up = −Z, down = +Z,
// left = −X, right = +X. It is (0, 0) when nothing is held.
func direction() (dx, dz float64) {
	if held["left"] {
		dx--
	}
	if held["right"] {
		dx++
	}
	if held["up"] {
		dz--
	}
	if held["down"] {
		dz++
	}
	if l := math.Hypot(dx, dz); l > 0 {
		dx, dz = dx/l, dz/l
	}
	return dx, dz
}
