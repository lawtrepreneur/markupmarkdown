package api_test

import (
	"encoding/json"
	"testing"

	"markupmarkdown/internal/testutil"
)

func TestListModelsAuthenticatedUsesConfiguredPolicy(t *testing.T) {
	srv, st, _ := newTestServer(t)
	user := testutil.NewTestUser(t, st)
	sess := testutil.NewTestSession(t, st, user.ID)

	status, body := doJSON(t, srv, "GET", "/api/models", nil, withCookie(sess))
	if status != 200 {
		t.Fatalf("models: status=%d body=%s", status, body)
	}
	var models []struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.Unmarshal(body, &models); err != nil {
		t.Fatal(err)
	}
	if len(models) == 0 {
		t.Fatal("configured policy returned no enabled models")
	}
	for _, model := range models {
		if model.ID == "" || model.Enabled {
			t.Fatalf("unexpected model response: %+v", model)
		}
		if model.ID == "disabled" {
			t.Fatal("disabled model returned")
		}
	}
}
