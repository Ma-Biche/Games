// Command server serves the Games repo statically and hosts the Forest Friends WebSocket at /ws.
package main

import (
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// checkRoot returns an error unless root/games/forest-friends/index.html is a regular file.
func checkRoot(root string) error {
	p := filepath.Join(root, "games", "forest-friends", "index.html")
	fi, err := os.Stat(p)
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return errors.New(p + " is not a regular file")
	}
	return nil
}

// staticHandler serves root, hiding any path segment that starts with '.'.
func staticHandler(root string) http.Handler {
	fs := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, seg := range strings.Split(r.URL.Path, "/") {
			if strings.HasPrefix(seg, ".") {
				http.NotFound(w, r)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-cache")
		fs.ServeHTTP(w, r)
	})
}

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	root := flag.String("root", "../..", "path to the Games repo root")
	flag.Parse()

	if err := checkRoot(*root); err != nil {
		log.Fatalf("root %q does not look like the Games repo (use -root): %v", *root, err)
	}

	hub := NewHub(uint64(time.Now().UnixNano()))
	go hub.Run(nil)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.ServeWS)
	mux.Handle("/", staticHandler(*root))

	log.Printf("Forest Friends: http://localhost%s/games/forest-friends/", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
