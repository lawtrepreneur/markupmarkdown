// Package matterrepo is the per-Matter Git layer: one repo per matter, one
// commit per accepted document revision. Shells out to the git binary.
package matterrepo

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
)

// ErrStaleParent: caller's parentSHA != current HEAD (map to HTTP 409).
var ErrStaleParent = errors.New("matterrepo: stale parent")

// ErrBadPath: file path absolute, contains "..", or targets .git.
var ErrBadPath = errors.New("matterrepo: invalid path")

// ErrBadSHA: sha is not a 40-char lowercase hex commit id.
var ErrBadSHA = errors.New("matterrepo: invalid sha")

var shaRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// CommitMeta is recorded as trailer lines in the commit message.
type CommitMeta struct {
	Matter            string
	Document          string
	Actor             string
	Session           string
	Operation         string
	RevisionID        string
	SerializerVersion string
	RevertOf          string
}

// Revision is one commit in a matter repository.
type Revision struct {
	SHA       string `json:"sha"`
	ParentSHA string `json:"parentSHA"`
	CreatedAt string `json:"createdAt"`
	Actor     string `json:"actor"`
	Operation string `json:"operation"`
	Message   string `json:"message"`
}

// Repo is a handle on one matter's git repository.
type Repo struct {
	root string
	mu   sync.Mutex // ponytail: per-process lock; multi-process needs git index.lock retry
}

// Init creates the repo at root if absent and returns a handle.
func Init(root string) (*Repo, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	r := &Repo{root: root}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		if _, err := r.git("init", "-q"); err != nil {
			return nil, err
		}
	}
	for k, v := range map[string]string{
		"user.name":      "markupmarkdown",
		"user.email":     "matter@markupmarkdown.local",
		"commit.gpgsign": "false",
	} {
		if _, err := r.git("config", k, v); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// Open is Init; kept for call-site readability.
func Open(root string) (*Repo, error) { return Init(root) }

func (r *Repo) git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.root
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

// Head returns HEAD sha, "" if no commits.
func (r *Repo) Log() ([]Revision, error) {
	out, err := r.git("log", "--format=%H%x00%P%x00%cI%x00%B%x00")
	if err != nil {
		return nil, err
	}
	parts := strings.Split(out, "\x00")
	var revisions []Revision
	for i := 0; i+3 < len(parts); i += 4 {
		message := strings.TrimSpace(parts[i+3])
		lines := strings.SplitN(message, "\n", 2)
		r := Revision{SHA: parts[i], CreatedAt: parts[i+2], Message: lines[0]}
		if parts[i+1] != "" {
			r.ParentSHA = strings.Fields(parts[i+1])[0]
		}
		for _, line := range strings.Split(message, "\n") {
			if strings.HasPrefix(line, "Actor: ") {
				r.Actor = strings.TrimPrefix(line, "Actor: ")
			}
			if strings.HasPrefix(line, "Operation: ") {
				r.Operation = strings.TrimPrefix(line, "Operation: ")
			}
		}
		revisions = append(revisions, r)
	}
	return revisions, nil
}

func (r *Repo) Head() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.head()
}

func (r *Repo) head() (string, error) {
	out, err := r.git("rev-parse", "--verify", "-q", "HEAD")
	if err != nil {
		// rev-parse -q exits 1 with no output when HEAD unborn.
		return "", nil
	}
	return strings.TrimSpace(out), nil
}

func cleanPath(p string) (string, error) {
	if p == "" || filepath.IsAbs(p) || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") {
		return "", ErrBadPath
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." || seg == ".git" {
			return "", ErrBadPath
		}
	}
	c := filepath.Clean(p)
	if c == "." || strings.HasPrefix(c, "..") {
		return "", ErrBadPath
	}
	return c, nil
}

// noSymlinks rejects p if any existing component under root is a symlink.
func (r *Repo) noSymlinks(p string) error {
	cur := r.root
	for _, seg := range strings.Split(filepath.ToSlash(p), "/") {
		cur = filepath.Join(cur, seg)
		fi, err := os.Lstat(cur)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlink in %q", ErrBadPath, p)
		}
	}
	return nil
}

// snapshot captures the pre-write state of one target path.
// nil = path did not exist before the commit attempt.
type snapshot struct {
	data []byte
	mode os.FileMode
}

// rollback restores each target path to its pre-write state and unstages it.
// Pre-existing untracked files are preserved; created empty dirs may remain.
func (r *Repo) rollback(head string, snap map[string]*snapshot) {
	paths := make([]string, 0, len(snap))
	for p := range snap {
		paths = append(paths, p)
	}
	// Targeted reset leaves unrelated index entries and worktree files alone.
	_, _ = r.git(append([]string{"reset", "-q", "HEAD", "--"}, paths...)...)
	for p, s := range snap {
		full := filepath.Join(r.root, p)
		if s == nil {
			_ = os.Remove(full)
			continue
		}
		_ = os.WriteFile(full, s.data, s.mode)
	}
}

// writeNoFollow writes data to path, refusing a symlink as final component.
// ponytail: O_NOFOLLOW covers the final component only; parent dirs rely on the
// noSymlinks recheck immediately before (small residual race); Linux/unix only.
func writeNoFollow(path string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|syscall.O_NOFOLLOW, perm)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadPath, err)
	}
	if err := f.Truncate(0); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func message(m CommitMeta, doc string) string {
	if m.Document == "" {
		m.Document = doc
	}
	subj := m.Operation
	if subj == "" {
		subj = "commit"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n\n", subj, m.Document)
	for _, kv := range [][2]string{
		{"Matter", m.Matter}, {"Document", m.Document}, {"Actor", m.Actor},
		{"Session", m.Session}, {"Operation", m.Operation},
		{"Revision-ID", m.RevisionID}, {"Serializer-Version", m.SerializerVersion},
		{"Revert-Of", m.RevertOf},
	} {
		if kv[1] != "" {
			fmt.Fprintf(&b, "%s: %s\n", kv[0], kv[1])
		}
	}
	return b.String()
}

// Commit writes files and commits them. parentSHA must equal HEAD.
func (r *Repo) Commit(doc, parentSHA string, files map[string][]byte, meta CommitMeta) (string, error) {
	paths := make([]string, 0, len(files))
	clean := make(map[string][]byte, len(files))
	for p, data := range files {
		c, err := cleanPath(p)
		if err != nil {
			return "", fmt.Errorf("%w: %q", err, p)
		}
		clean[c] = data
		paths = append(paths, c)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	h, err := r.head()
	if err != nil {
		return "", err
	}
	if h != parentSHA {
		return "", ErrStaleParent
	}
	// Snapshot pre-write state of every target path.
	snap := make(map[string]*snapshot, len(clean))
	for p := range clean {
		if err := r.noSymlinks(p); err != nil {
			return "", err
		}
		full := filepath.Join(r.root, p)
		fi, err := os.Lstat(full)
		if os.IsNotExist(err) {
			snap[p] = nil // absent → remove on rollback
		} else if err != nil {
			return "", err
		} else {
			data, err := os.ReadFile(full)
			if err != nil {
				return "", err
			}
			snap[p] = &snapshot{data: data, mode: fi.Mode().Perm()}
		}
	}
	werr := func() error {
		for p, data := range clean {
			full := filepath.Join(r.root, p)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return err
			}
			// Recheck after MkdirAll: a component may have been swapped for a symlink.
			if err := r.noSymlinks(p); err != nil {
				return err
			}
			if err := writeNoFollow(full, data, 0o644); err != nil {
				return err
			}
		}
		if len(paths) > 0 {
			if _, err := r.git(append([]string{"add", "--"}, paths...)...); err != nil {
				return err
			}
		}
		_, err := r.git("commit", "-q", "--allow-empty", "-m", message(meta, doc))
		return err
	}()
	if werr != nil {
		r.rollback(h, snap)
		return "", werr
	}
	return r.head()
}

// Diff returns unified diff from..to for path ("" = all). Empty from = empty tree.
func (r *Repo) Diff(fromSHA, toSHA, path string) (string, error) {
	if fromSHA == "" {
		fromSHA = emptyTree
	}
	if fromSHA != emptyTree && !shaRe.MatchString(fromSHA) || !shaRe.MatchString(toSHA) {
		return "", ErrBadSHA
	}
	args := []string{"diff", fromSHA, toSHA}
	if path != "" {
		c, err := cleanPath(path)
		if err != nil {
			return "", err
		}
		args = append(args, "--", c)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.git(args...)
}

// Revert adds a new inverse commit of targetSHA on top of HEAD. parentSHA must equal HEAD.
func (r *Repo) Revert(doc, parentSHA, targetSHA string, meta CommitMeta) (string, error) {
	if !shaRe.MatchString(targetSHA) {
		return "", ErrBadSHA
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	h, err := r.head()
	if err != nil {
		return "", err
	}
	if h != parentSHA {
		return "", ErrStaleParent
	}
	if _, err := r.git("cat-file", "-e", targetSHA+"^{commit}"); err != nil {
		return "", fmt.Errorf("%w: %v", ErrBadSHA, err)
	}
	abort := func() {
		_, _ = r.git("revert", "--abort")
		_, _ = r.git("reset", "-q", "--hard", "HEAD")
	}
	if _, err := r.git("revert", "--no-commit", targetSHA); err != nil {
		abort()
		return "", err
	}
	meta.RevertOf = targetSHA
	if meta.Operation == "" {
		meta.Operation = "revert"
	}
	if _, err := r.git("commit", "-q", "--allow-empty", "-m", message(meta, doc)); err != nil {
		abort()
		return "", err
	}
	return r.head()
}

// ReadFileAt returns file content at commit sha.
func (r *Repo) ReadFileAt(sha, path string) ([]byte, error) {
	if !shaRe.MatchString(sha) {
		return nil, ErrBadSHA
	}
	c, err := cleanPath(path)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out, err := r.git("show", sha+":"+filepath.ToSlash(c))
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}
