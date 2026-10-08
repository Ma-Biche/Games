# Forest Friends

A small multiplayer game in Go: the client is compiled to WebAssembly (WebGL), the server is a stdlib Go HTTP + WebSocket server. Collect stars in the forest.

All commands run in `games/forest-friends`.

## Build

```sh
make build
```

PowerShell equivalent:

```powershell
$env:GOOS="js"; $env:GOARCH="wasm"; go build -trimpath -ldflags="-s -w" -o main.wasm ./client
Copy-Item "$(go env GOROOT)\lib\wasm\wasm_exec.js" .
```

`main.wasm` and `wasm_exec.js` must come from the same Go version. Rebuild and commit them together.

## Run

```sh
make run        # same as: go run ./server
```

Open http://localhost:8080/games/forest-friends/.

Flags: `-addr` (listen address, default `:8080`) and `-root` (path to the Games repo root, default `../..`), e.g. `go run ./server -addr :9000 -root ../..`.

## Test

```sh
make test
make vet
```

## Notes

- Static hosting (no Go server) works: the game runs offline, single player.
- `file://` does not work, because browsers block fetching the wasm. Use a web server.
- Add `?offline=1` to the URL to force offline single-player mode.
- The `/ws` endpoint does not check the `Origin` header.

## Controls

WASD or arrow keys. On touch screens, use the on-screen pad.
