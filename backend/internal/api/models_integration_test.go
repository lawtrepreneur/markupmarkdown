package api_test

import (
	"encoding/json"
	"os"
	"testing"

	"markupmarkdown/internal/testutil"
)

func TestListModelsAuthenticatedUsesConfiguredPolicy(t *testing.T) {
	// Use the repo example policy: 2 enabled, 1 disabled. t.Setenv works
	// only if config loads after this, but config is loaded once in
	// TestMain — so this test asserts against modelpolicy.example.json.
	policyPath := "../../modelpolicy.example.json"
	if _, err := os.Stat(policyPath); err != nil {
		t.Skipf("example policy missing: %v", err)
	}
	t.Setenv("MODEL_POLICY_PATH", policyPath)

	srv, st, _ := newTestServer(t)
	user := testutil.NewTestUser(t, st)
	sess := testutil.NewTestSession(t, st, user.ID)

	status, body := doJSON(t, srv, "GET", "/api/models", nil, withCookie(sess))
	if status != 200 {
		t.Fatalf("models: status=%d body=%s", status, body)
	}
	var models []struct {
		ID       string `json:"id"`
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(body, &models); err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 enabled models, got %d: %s", len(models), body)
	}
	if models[0].ID != "claude-sonnet-4-6" || models[1].ID != "local-llama" {
		t.Fatalf("unexpected models: %s", body)
	}
}
