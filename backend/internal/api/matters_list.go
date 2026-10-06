package api

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"sort"
	"strings"
)

func (a *API) listMatters(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	names, err := matterNames(a.cfg.OpenCode.MattersDir)
	if err != nil {
		internalError(w, "listMatters", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]string{"matters": names})
}

func matterNames(dir string) ([]string, error) {
	names := []string{}
	if dir == "" {
		return names, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return names, nil
		}
		return nil, err
	}
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() || strings.HasPrefix(n, ".") || !safeMatterID(n) {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}
