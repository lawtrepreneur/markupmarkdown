package api_test

// Tests for the matter-revisions HTTP handlers (matter_revisions.go).
// Requires MONGODB_URI (integration suite, like the rest of api tests);
// git binary must be on PATH (matterrepo shells out to it).

import (
	"strings"
	"testing"

	"markupmarkdown/internal/testutil"
)

// commitReqBody is a small helper building a commit request body.
func commitReqBody(matterID, parentSHA, path, content string) map[string]any {
	return map[string]any{
		"matterId":  matterID,
		"parentSHA": parentSHA,
		"files":     map[string]string{path: content},
		"meta": map[string]any{
			"actor":             "attacker-should-be-ignored",
			"session":           "s1",
			"operation":         "save",
			"revisionId":        "r1",
			"serializerVersion": "1",
		},
	}
}

func TestMatterRevisionsUnauthenticated(t *testing.T) {
	srv, st, _ := newTestServer(t)
	u := testutil.NewTestUser(t, st)
	doc := testutil.NewTestDocument(t, st, u.ID, "")

	code, _ := doJSON(t, srv, "POST", "/api/documents/"+doc.ID+"/matter-revisions",
		commitReqBody("m1", "", "a.md", "x"))
	if code != 401 {
		t.Fatalf("want 401 unauth, got %d", code)
	}
}

func TestMatterRevisionsStaleParent(t *testing.T) {
	srv, st, api := newTestServer(t)
	api.SetMattersDirForTest(t.TempDir())
	u := testutil.NewTestUser(t, st)
	doc := testutil.NewTestDocument(t, st, u.ID, "")
	sid := testutil.NewTestSession(t, st, u.ID)
	url := "/api/documents/" + doc.ID + "/matter-revisions"

	code, body := doJSON(t, srv, "POST", url, commitReqBody("m-stale", "", "a.md", "v1"), withCookie(sid))
	if code != 201 {
		t.Fatalf("first commit: want 201, got %d (%s)", code, body)
	}
	// Second commit with stale parent (empty again instead of the new HEAD).
	code, body = doJSON(t, srv, "POST", url, commitReqBody("m-stale", "", "a.md", "v2"), withCookie(sid))
	if code != 409 {
		t.Fatalf("stale parent: want 409, got %d (%s)", code, body)
	}
}

func TestMatterRevisionsBadMatterID(t *testing.T) {
	srv, st, api := newTestServer(t)
	api.SetMattersDirForTest(t.TempDir())
	u := testutil.NewTestUser(t, st)
	doc := testutil.NewTestDocument(t, st, u.ID, "")
	sid := testutil.NewTestSession(t, st, u.ID)
	url := "/api/documents/" + doc.ID + "/matter-revisions"

	for _, bad := range []string{"../escape", "a/b", "", "a\\b"} {
		code, _ := doJSON(t, srv, "POST", url, commitReqBody(bad, "", "a.md", "x"), withCookie(sid))
		if code != 400 {
			t.Fatalf("matterId %q: want 400, got %d", bad, code)
		}
	}
	// Bad file path too.
	code, _ := doJSON(t, srv, "POST", url, commitReqBody("m-bad", "", "../evil.md", "x"), withCookie(sid))
	if code != 400 {
		t.Fatalf("bad path: want 400, got %d", code)
	}
}

func TestMatterRevisionsDiffAndRevert(t *testing.T) {
	srv, st, api := newTestServer(t)
	api.SetMattersDirForTest(t.TempDir())
	u := testutil.NewTestUser(t, st)
	doc := testutil.NewTestDocument(t, st, u.ID, "")
	sid := testutil.NewTestSession(t, st, u.ID)
	base := "/api/documents/" + doc.ID + "/matter-revisions"

	var first struct {
		SHA string `json:"sha"`
	}
	code, body := doJSON(t, srv, "POST", base, commitReqBody("m-dr", "", "a.md", "v1"), withCookie(sid))
	if code != 201 {
		t.Fatalf("commit1: want 201, got %d (%s)", code, body)
	}
	mustDecode(t, body, &first)

	var second struct {
		SHA string `json:"sha"`
	}
	code, body = doJSON(t, srv, "POST", base,
		commitReqBody("m-dr", first.SHA, "a.md", "v2"), withCookie(sid))
	if code != 201 {
		t.Fatalf("commit2: want 201, got %d (%s)", code, body)
	}
	mustDecode(t, body, &second)

	// Diff from first to second should mention a.md.
	code, body = doJSON(t, srv, "GET",
		base+"/diff?matterId=m-dr&from="+first.SHA+"&to="+second.SHA, nil, withCookie(sid))
	if code != 200 || !strings.Contains(string(body), "a.md") {
		t.Fatalf("diff: want 200 with a.md, got %d (%s)", code, body)
	}

	// Revert second commit.
	code, body = doJSON(t, srv, "POST", base+"/"+second.SHA+"/revert",
		map[string]any{"matterId": "m-dr", "parentSHA": second.SHA}, withCookie(sid))
	if code != 201 {
		t.Fatalf("revert: want 201, got %d (%s)", code, body)
	}

	// Stale revert (same parent again) → 409.
	code, _ = doJSON(t, srv, "POST", base+"/"+second.SHA+"/revert",
		map[string]any{"matterId": "m-dr", "parentSHA": second.SHA}, withCookie(sid))
	if code != 409 {
		t.Fatalf("stale revert: want 409, got %d", code)
	}
}
