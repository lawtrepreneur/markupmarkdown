package opencode

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrPathEscape = errors.New("opencode: selected path escapes matter root")

// Manifest is built once per review; fields are private and slices are
// copied in and out, so callers cannot mutate it.
type Manifest struct {
	matterID      string
	sessionID     string
	selectedPaths []string
	readFiles     []string
	readTracking  string
	createdAt     time.Time
}

func newManifest(matterID, sessionID string, selected []string) Manifest {
	return Manifest{
		matterID: matterID, sessionID: sessionID,
		selectedPaths: append([]string(nil), selected...),
		readFiles:     []string{}, readTracking: "incomplete", createdAt: time.Now().UTC(),
	}
}

func (m Manifest) MatterID() string        { return m.matterID }
func (m Manifest) SessionID() string       { return m.sessionID }
func (m Manifest) SelectedPaths() []string { return append([]string(nil), m.selectedPaths...) }
func (m Manifest) ReadFiles() []string     { return append([]string{}, m.readFiles...) }
func (m Manifest) ReadTracking() string    { return m.readTracking }
func (m Manifest) CreatedAt() time.Time    { return m.createdAt }

// validatePaths rejects absolute, "..", and symlink-escaping paths.
func validatePaths(root string, paths []string) error {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	for _, p := range paths {
		if p == "" || filepath.IsAbs(p) {
			return fmt.Errorf("%w: %q", ErrPathEscape, p)
		}
		for _, seg := range strings.Split(filepath.ToSlash(p), "/") {
			if seg == ".." {
				return fmt.Errorf("%w: %q", ErrPathEscape, p)
			}
		}
		real, err := filepath.EvalSymlinks(filepath.Join(realRoot, p))
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(realRoot, real)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("%w: %q", ErrPathEscape, p)
		}
	}
	return nil
}
