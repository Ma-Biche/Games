package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckRoot(t *testing.T) {
	if err := checkRoot("../../.."); err != nil {
		t.Fatalf("checkRoot(repo) = %v", err)
	}
	if err := checkRoot(t.TempDir()); err == nil {
		t.Fatal("checkRoot(empty dir) = nil, want error")
	}
}

func TestStaticHandler(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{"a.txt": "hello", ".git/config": "secret", "a/.hidden": "secret"}
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h := staticHandler(root)
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}
	for _, p := range []string{"/.git/config", "/a/.hidden"} {
		if w := get(p); w.Code != http.StatusNotFound {
			t.Errorf("%s: code %d, want 404", p, w.Code)
		}
	}
	w := get("/a.txt")
	if w.Code != http.StatusOK || w.Body.String() != "hello" {
		t.Errorf("/a.txt: code %d body %q", w.Code, w.Body.String())
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
}
