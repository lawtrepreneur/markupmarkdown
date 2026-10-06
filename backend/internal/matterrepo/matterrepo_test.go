package matterrepo

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newRepo(t *testing.T) *Repo {
	t.Helper()
	r, err := Init(t.TempDir())
	if err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	return r
}

func commit(t *testing.T, r *Repo, parent, body string) string {
	t.Helper()
	sha, err := r.Commit("a", parent, map[string][]byte{
		"drafts/a.md":   []byte(body),
		"drafts/a.json": []byte(`{}`),
	}, CommitMeta{Matter: "m1", Actor: "u", Operation: "save", RevisionID: "r1", SerializerVersion: "1"})
	if err != nil {
		t.Fatal(err)
	}
	return sha
}

func TestInitCommitHead(t *testing.T) {
	r := newRepo(t)
	if h, _ := r.Head(); h != "" {
		t.Fatalf("head=%q want empty", h)
	}
	sha := commit(t, r, "", "one\n")
	if h, _ := r.Head(); h != sha || sha == "" {
		t.Fatalf("head=%q sha=%q", h, sha)
	}
	msg, _ := r.git("log", "-1", "--format=%B")
	for _, want := range []string{"Matter: m1", "Document: a", "Actor: u", "Revision-ID: r1", "Serializer-Version: 1"} {
		if !strings.Contains(msg, want) {
			t.Errorf("msg missing %q: %s", want, msg)
		}
	}
}

func TestStaleParent(t *testing.T) {
	r := newRepo(t)
	commit(t, r, "", "one\n")
	_, err := r.Commit("a", "", map[string][]byte{"drafts/a.md": []byte("x")}, CommitMeta{})
	if !errors.Is(err, ErrStaleParent) {
		t.Fatalf("err=%v", err)
	}
}

func TestPathTraversal(t *testing.T) {
	r := newRepo(t)
	for _, p := range []string{"../x", "a/../../x", "/etc/x", ".git/config", ""} {
		_, err := r.Commit("a", "", map[string][]byte{p: []byte("x")}, CommitMeta{})
		if !errors.Is(err, ErrBadPath) {
			t.Errorf("%q: err=%v", p, err)
		}
	}
}

func TestDiffRevertReadFileAt(t *testing.T) {
	r := newRepo(t)
	s1 := commit(t, r, "", "one\n")
	s2 := commit(t, r, s1, "two\n")

	d, err := r.Diff(s1, s2, "drafts/a.md")
	if err != nil || !strings.Contains(d, "-one") || !strings.Contains(d, "+two") {
		t.Fatalf("diff=%q err=%v", d, err)
	}
	d0, err := r.Diff("", s1, "drafts/a.md")
	if err != nil || !strings.Contains(d0, "+one") {
		t.Fatalf("empty-tree diff=%q err=%v", d0, err)
	}

	old, err := r.ReadFileAt(s1, "drafts/a.md")
	if err != nil || string(old) != "one\n" {
		t.Fatalf("old=%q err=%v", old, err)
	}

	s3, err := r.Revert("a", s2, s2, CommitMeta{Actor: "u"})
	if err != nil || s3 == s2 {
		t.Fatalf("revert sha=%q err=%v", s3, err)
	}
	cur, _ := r.ReadFileAt(s3, "drafts/a.md")
	if string(cur) != "one\n" {
		t.Fatalf("cur=%q", cur)
	}
	n, _ := r.git("rev-list", "--count", "HEAD")
	if strings.TrimSpace(n) != "3" {
		t.Fatalf("count=%s want 3", n)
	}
	msg, _ := r.git("log", "-1", "--format=%B")
	if !strings.Contains(msg, "Revert-Of: "+s2) {
		t.Fatalf("msg=%s", msg)
	}
}

func TestSymlinkEscape(t *testing.T) {
	r := newRepo(t)
	out := t.TempDir()
	if err := os.Symlink(out, filepath.Join(r.root, "link")); err != nil {
		t.Skip(err)
	}
	_, err := r.Commit("a", "", map[string][]byte{"link/x.md": []byte("x")}, CommitMeta{})
	if !errors.Is(err, ErrBadPath) {
		t.Fatalf("dir symlink err=%v", err)
	}
	target := filepath.Join(out, "t")
	_ = os.WriteFile(target, []byte("orig"), 0o644)
	_ = os.Symlink(target, filepath.Join(r.root, "f.md"))
	_, err = r.Commit("a", "", map[string][]byte{"f.md": []byte("x")}, CommitMeta{})
	if !errors.Is(err, ErrBadPath) {
		t.Fatalf("file symlink err=%v", err)
	}
	if b, _ := os.ReadFile(target); string(b) != "orig" {
		t.Fatalf("escaped write: %q", b)
	}
}

func TestCommitFailureCleans(t *testing.T) {
	r := newRepo(t)
	hook := filepath.Join(r.root, ".git", "hooks", "pre-commit")
	if err := os.MkdirAll(filepath.Dir(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755)
	if _, err := r.Commit("a", "", map[string][]byte{"drafts/a.md": []byte("x")}, CommitMeta{}); err == nil {
		t.Fatal("want error")
	}
	if st, _ := r.git("status", "--porcelain"); strings.TrimSpace(st) != "" {
		t.Fatalf("dirty: %q", st)
	}
}

func failHook(t *testing.T, r *Repo) {
	t.Helper()
	hook := filepath.Join(r.root, ".git", "hooks", "pre-commit")
	_ = os.MkdirAll(filepath.Dir(hook), 0o755)
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestFailedCommitKeepsUntracked(t *testing.T) {
	for _, withHead := range []bool{false, true} {
		r := newRepo(t)
		parent := ""
		if withHead {
			parent = commit(t, r, "", "one\n")
		}
		p := filepath.Join(r.root, "drafts", "u.md")
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("mine"), 0o644)
		failHook(t, r)
		if _, err := r.Commit("a", parent, map[string][]byte{"drafts/u.md": []byte("new")}, CommitMeta{}); err == nil {
			t.Fatal("want error")
		}
		if b, _ := os.ReadFile(p); string(b) != "mine" {
			t.Fatalf("head=%v untracked lost: %q", withHead, b)
		}
		if st, _ := r.git("diff", "--cached", "--name-only"); strings.TrimSpace(st) != "" {
			t.Fatalf("head=%v staged: %q", withHead, st)
		}
	}
}

func TestFailedCommitRestoresTracked(t *testing.T) {
	r := newRepo(t)
	s1 := commit(t, r, "", "one\n")
	failHook(t, r)
	if _, err := r.Commit("a", s1, map[string][]byte{"drafts/a.md": []byte("two\n")}, CommitMeta{}); err == nil {
		t.Fatal("want error")
	}
	if b, _ := os.ReadFile(filepath.Join(r.root, "drafts/a.md")); string(b) != "one\n" {
		t.Fatalf("tracked=%q", b)
	}
	if st, _ := r.git("status", "--porcelain"); strings.TrimSpace(st) != "" {
		t.Fatalf("dirty: %q", st)
	}
}

func TestRevertUnknownSHA(t *testing.T) {
	r := newRepo(t)
	s1 := commit(t, r, "", "one\n")
	unknown := strings.Repeat("0", 40)
	if _, err := r.Revert("a", s1, unknown, CommitMeta{}); !errors.Is(err, ErrBadSHA) {
		t.Fatalf("err=%v", err)
	}
}

func TestRevertStaleAndConflict(t *testing.T) {
	r := newRepo(t)
	s1 := commit(t, r, "", "one\n")
	s2 := commit(t, r, s1, "two\n")
	s3 := commit(t, r, s2, "three\n")
	if _, err := r.Revert("a", s1, s2, CommitMeta{}); !errors.Is(err, ErrStaleParent) {
		t.Fatalf("stale err=%v", err)
	}
	if _, err := r.Revert("a", s3, s2, CommitMeta{}); err == nil {
		t.Fatal("want conflict")
	}
	if st, _ := r.git("status", "--porcelain"); strings.TrimSpace(st) != "" {
		t.Fatalf("dirty: %q", st)
	}
	if h, _ := r.Head(); h != s3 {
		t.Fatalf("head moved")
	}
}

func TestBadSHA(t *testing.T) {
	r := newRepo(t)
	s1 := commit(t, r, "", "one\n")
	for _, bad := range []string{"--output=x", "HEAD", s1[:7]} {
		if _, err := r.Diff(bad, s1, ""); !errors.Is(err, ErrBadSHA) {
			t.Errorf("diff %q err=%v", bad, err)
		}
		if _, err := r.ReadFileAt(bad, "drafts/a.md"); !errors.Is(err, ErrBadSHA) {
			t.Errorf("read %q err=%v", bad, err)
		}
		if _, err := r.Revert("a", s1, bad, CommitMeta{}); !errors.Is(err, ErrBadSHA) {
			t.Errorf("revert %q err=%v", bad, err)
		}
	}
}
