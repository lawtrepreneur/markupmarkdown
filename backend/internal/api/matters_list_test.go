package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"markupmarkdown/internal/config"
)

func TestListMattersRequiresAuth(t *testing.T) {
	a := &API{cfg: &config.Config{}}
	w := httptest.NewRecorder()
	a.listMatters(w, httptest.NewRequest("GET", "/api/matters", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestListMattersFilters(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b", "a", ".hidden", "sub"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "link"), []byte("nowhere"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("sub", filepath.Join(dir, "sym")); err != nil {
		t.Fatal(err)
	}
	got, err := matterNames(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "b", "sub"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestListMattersUnsetAndMissing(t *testing.T) {
	for _, dir := range []string{"", filepath.Join(t.TempDir(), "missing")} {
		got, err := matterNames(dir)
		if err != nil || len(got) != 0 {
			t.Fatalf("dir=%q: got=%v err=%v", dir, got, err)
		}
	}
}
