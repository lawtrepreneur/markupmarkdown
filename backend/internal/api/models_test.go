package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"markupmarkdown/internal/config"
)

func TestListModelsRequiresAuth(t *testing.T) {
	a := &API{cfg: &config.Config{}}
	w := httptest.NewRecorder()
	a.listModels(w, httptest.NewRequest("GET", "/api/models", nil))
	if w.Code != 401 {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func modelsFor(t *testing.T, path string) (int, []modelResponse) {
	t.Helper()
	a := &API{cfg: &config.Config{}}
	w := httptest.NewRecorder()
	a.writeModels(w, path)
	var out []modelResponse
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return w.Code, out
}

func TestListModelsPolicy(t *testing.T) {
	p := filepath.Join(t.TempDir(), "p.json")
	body := `{"models":[{"id":"a","provider":"x","external":true,"enabled":true},{"id":"b","provider":"y","enabled":false}]}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out := modelsFor(t, p)
	if code != 200 || len(out) != 1 || out[0] != (modelResponse{ID: "a", Provider: "x", External: true}) {
		t.Fatalf("code=%d out=%v", code, out)
	}
}

func TestListModelsUnsetPath(t *testing.T) {
	code, out := modelsFor(t, "")
	if code != 200 || out == nil || len(out) != 0 {
		t.Fatalf("code=%d out=%v", code, out)
	}
}

func TestConfigModelPolicyEnv(t *testing.T) {
	t.Setenv("MODEL_POLICY_PATH", "/x/p.json")
	if v := os.Getenv("MODEL_POLICY_PATH"); v == "" {
		t.Fatal("env")
	}
}
