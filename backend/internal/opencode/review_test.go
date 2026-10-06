package opencode

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fake(t *testing.T, reply string, delay time.Duration) (*Client, *int32, *int32) {
	var created, deleted int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/session":
			atomic.AddInt32(&created, 1)
			w.Write([]byte(`{"id":"s1"}`))
		case r.Method == "DELETE" && r.URL.Path == "/session/s1":
			atomic.AddInt32(&deleted, 1)
			w.Write([]byte(`true`))
		case r.Method == "POST" && r.URL.Path == "/session/s1/message":
			select {
			case <-time.After(delay):
			case <-r.Context().Done():
				return
			}
			w.Write([]byte(`{"parts":[{"type":"text","text":` + reply + `}]}`))
		}
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL), &created, &deleted
}

func TestPathEscapeRejected(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(outside, "x"), nil, 0o644)
	os.Symlink(outside, filepath.Join(root, "link"))
	c, created, _ := fake(t, `"[]"`, 0)
	for _, p := range []string{"../x", "/etc/passwd", "link/x"} {
		_, err := c.RunReview(context.Background(), "m1", root, []string{p}, "tighten_prose", "a/b", "doc")
		if !errors.Is(err, ErrPathEscape) {
			t.Fatalf("%q: err=%v", p, err)
		}
	}
	if *created != 0 {
		t.Fatal("session created before validation")
	}
}

func TestTimeoutClosesSession(t *testing.T) {
	c, _, deleted := fake(t, `"[]"`, time.Minute)
	_, err := c.runReview(context.Background(), 50*time.Millisecond, "m1", t.TempDir(), nil, "tighten_prose", "a/b", "doc")
	if err == nil {
		t.Fatal("want timeout error")
	}
	if atomic.LoadInt32(deleted) != 1 {
		t.Fatal("session not closed")
	}
}

func TestInvalidQuotedDropped(t *testing.T) {
	reply := `"[{\"quoted\":\"hello\",\"replacement\":\"hi\",\"rationale\":\"r\"},{\"quoted\":\"nope\",\"replacement\":\"x\",\"rationale\":\"r\"}]"`
	c, _, deleted := fake(t, reply, 0)
	res, err := c.RunReview(context.Background(), "m1", t.TempDir(), nil, "tighten_prose", "a/b", "say hello world")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Suggestions) != 1 || res.Dropped != 1 || res.Manifest.ReadTracking() != "incomplete" || *deleted != 1 {
		t.Fatalf("%+v deleted=%d", res, *deleted)
	}
}

func TestManifestImmutable(t *testing.T) {
	sel := []string{"a"}
	m := newManifest("m1", "s1", sel)
	sel[0] = "x"
	m.SelectedPaths()[0] = "y"
	m.ReadFiles()
	_ = append(m.ReadFiles(), "z")
	if m.SelectedPaths()[0] != "a" || len(m.ReadFiles()) != 0 || m.MatterID() != "m1" {
		t.Fatalf("%+v", m)
	}
}

func TestAmbiguousQuotedDropped(t *testing.T) {
	reply := `"[{\"quoted\":\"foo\",\"replacement\":\"x\",\"rationale\":\"r\"}]"`
	c, _, _ := fake(t, reply, 0)
	res, err := c.RunReview(context.Background(), "m1", t.TempDir(), nil, "tighten_prose", "a/b", "foo and foo")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Suggestions) != 0 || res.Ambiguous != 1 || res.Dropped != 0 {
		t.Fatalf("%+v", res)
	}
}

func TestUnknownPreset(t *testing.T) {
	c, created, _ := fake(t, `"[]"`, 0)
	_, err := c.RunReview(context.Background(), "m1", t.TempDir(), nil, "nope", "a/b", "d")
	if err == nil || *created != 0 {
		t.Fatalf("err=%v created=%d", err, *created)
	}
}

func TestNon2xxAndMalformed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/session" {
			http.Error(w, "boom", 500)
			return
		}
		w.Write([]byte("{not json"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	_, err := c.CreateSession(context.Background(), "/p")
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("want 500 err, got %v", err)
	}
	_, err = c.SendReview(context.Background(), "s1", "p")
	if err == nil {
		t.Fatal("want decode error")
	}
}

func TestModelSplitAndDirectory(t *testing.T) {
	var gotModel, gotDir string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/session" {
			gotDir = r.URL.Query().Get("directory")
			w.Write([]byte(`{"id":"s1"}`))
			return
		}
		b, _ := io.ReadAll(r.Body)
		gotModel = string(b)
		w.Write([]byte(`{"parts":[]}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	if _, err := c.CreateSession(context.Background(), "/my proj"); err != nil || gotDir != "/my proj" {
		t.Fatalf("dir=%q err=%v", gotDir, err)
	}
	if _, err := c.send(context.Background(), "s1", "anthropic/claude-x", "p"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotModel, `"providerID":"anthropic"`) || !strings.Contains(gotModel, `"modelID":"claude-x"`) {
		t.Fatalf("body=%s", gotModel)
	}
}
