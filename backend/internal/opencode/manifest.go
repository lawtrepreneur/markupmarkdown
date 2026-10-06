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

// Manifest is built once per review and never mutated.
type Manifest struct {
	MatterID      string
	SessionID     string
	SelectedPaths []string
	ReadFiles     []string
	ReadTracking  string
	CreatedAt     time.Time
}

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
