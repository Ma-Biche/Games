//go:build js && wasm

package main

import (
	"errors"
	"fmt"
	"syscall/js"
)

type settled struct {
	v   js.Value
	err error
}

// await blocks the main goroutine until the Promise p settles.
func await(p js.Value) (js.Value, error) {
	ch := make(chan settled, 1)
	onOK := js.FuncOf(func(this js.Value, args []js.Value) any {
		v := js.Undefined()
		if len(args) > 0 {
			v = args[0]
		}
		ch <- settled{v: v}
		return nil
	})
	onErr := js.FuncOf(func(this js.Value, args []js.Value) any {
		reason := "unknown error"
		if len(args) > 0 {
			reason = js.Global().Call("String", args[0]).String()
		}
		ch <- settled{v: js.Undefined(), err: errors.New(reason)}
		return nil
	})
	p.Call("then", onOK, onErr)
	r := <-ch
	onOK.Release()
	onErr.Release()
	return r.v, r.err
}

// startFetch starts a fetch and returns its Promise without awaiting it.
func startFetch(url string) js.Value {
	return js.Global().Call("fetch", url)
}

// readBytes reads the body of a fetch Response into a Go byte slice.
func readBytes(resp js.Value, url string) ([]byte, error) {
	if !resp.Get("ok").Bool() {
		return nil, fmt.Errorf("Could not load %s (HTTP %d)", url, resp.Get("status").Int())
	}
	buf, err := await(resp.Call("arrayBuffer"))
	if err != nil {
		return nil, fmt.Errorf("Could not load %s", url)
	}
	u8 := js.Global().Get("Uint8Array").New(buf)
	data := make([]byte, u8.Get("length").Int())
	js.CopyBytesToGo(data, u8)
	return data, nil
}

// loadImage loads and decodes an image.
func loadImage(url string) (js.Value, error) {
	img := js.Global().Get("Image").New()
	img.Set("src", url)
	if _, err := await(img.Call("decode")); err != nil {
		return js.Undefined(), fmt.Errorf("Could not load %s", url)
	}
	return img, nil
}
