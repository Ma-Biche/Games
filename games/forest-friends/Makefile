.PHONY: build run test vet
build:
	GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o main.wasm ./client
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" . 2>/dev/null || cp "$$(go env GOROOT)/misc/wasm/wasm_exec.js" .
run:
	go run ./server
test:
	go test ./game/... ./gfx/... ./server/...
vet:
	go vet ./game/... ./gfx/... ./server/... && GOOS=js GOARCH=wasm go vet ./client
