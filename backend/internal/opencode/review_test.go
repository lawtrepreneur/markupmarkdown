package opencode

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
		_, err := c.RunReview(context.Background(), root, []string{p}, "tighten_prose", "a/b", "doc")
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
	_, err := c.runReview(context.Background(), 50*time.Millisecond, t.TempDir(), nil, "tighten_prose", "a/b", "doc")
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
	res, err := c.RunReview(context.Background(), t.TempDir(), nil, "tighten_prose", "a/b", "say hello world")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Suggestions) != 1 || res.Dropped != 1 || res.Manifest.ReadTracking != "incomplete" || *deleted != 1 {
		t.Fatalf("%+v deleted=%d", res, *deleted)
	}
}
