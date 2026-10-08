//go:build js && wasm

package main

import (
	"sort"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/Ma-Biche/Games/games/forest-friends/game"
)

const starIcon = "../../assets/Vector/minimap_icon_star_yellow.svg"

var statusIcons = map[string]string{
	"connecting": "../../assets/Vector/round_grey.svg",
	"online":     "../../assets/Vector/round_grey_detailed_green.svg",
	"offline":    "../../assets/Vector/round_grey_detailed_red.svg",
}

var (
	doc                                js.Value
	canvas, loadingEl, loadErrEl       js.Value
	statusEl, statusIcon               js.Value
	scoreboard, starsLayer, padButtons js.Value
	stopped                            bool // set by fatal: no further frames
	prevSelfScore                      int
	havePrevSelf                       bool
)

func consoleCall(method string, args ...any) {
	js.Global().Get("console").Call(method, args...)
}

func setStatus(kind, text string) {
	statusIcon.Set("src", statusIcons[kind])
	statusEl.Set("textContent", text)
}

func el(tag, class, text string) js.Value {
	e := doc.Call("createElement", tag)
	if class != "" {
		e.Set("className", class)
	}
	if text != "" {
		e.Set("textContent", text)
	}
	return e
}

// renderScores rebuilds the scoreboard: score descending, then id.
func renderScores(players []game.PlayerState, selfID int) {
	rows := append([]game.PlayerState(nil), players...)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Score != rows[j].Score {
			return rows[i].Score > rows[j].Score
		}
		return rows[i].ID < rows[j].ID
	})
	scoreboard.Set("textContent", "")
	foundSelf := false
	for _, p := range rows {
		li := el("li", "", "")
		img := el("img", "", "")
		img.Set("src", starIcon)
		img.Set("alt", "")
		name := p.Animal
		if name != "" {
			name = strings.ToUpper(name[:1]) + name[1:]
		}
		if p.ID == selfID {
			name += " (you)"
		}
		li.Call("append", img, el("span", "name", name), el("span", "score", strconv.Itoa(p.Score)))
		scoreboard.Call("appendChild", li)
		if p.ID != selfID {
			continue
		}
		foundSelf = true
		if havePrevSelf && p.Score > prevSelfScore {
			bump(li)
		}
		prevSelfScore, havePrevSelf = p.Score, true
	}
	if !foundSelf {
		havePrevSelf = false
	}
}

// bump adds the bump class to li for 300 ms.
func bump(li js.Value) {
	li.Get("classList").Call("add", "bump")
	var done js.Func
	done = js.FuncOf(func(this js.Value, args []js.Value) any {
		li.Get("classList").Call("remove", "bump")
		done.Release()
		return nil
	})
	js.Global().Call("setTimeout", done, 300)
}

// fatal shows the loading overlay with msg and stops the frame loop.
func fatal(msg string, warn bool) {
	loadingEl.Get("style").Set("display", "")
	loadErrEl.Set("textContent", msg)
	if warn {
		consoleCall("warn", msg)
	} else {
		consoleCall("error", msg)
	}
	stopped = true
}
