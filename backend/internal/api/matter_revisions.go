package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"

	"markupmarkdown/internal/matterrepo"
	"markupmarkdown/internal/models"
)

type matterRevisionRequest struct {
	MatterID  string            `json:"matterId"`
	ParentSHA string            `json:"parentSHA"`
	Files     map[string]string `json:"files"`
	Meta      struct {
		Actor             string `json:"actor"` // ignored: forced server-side
		Session           string `json:"session"`
		Operation         string `json:"operation"`
		RevisionID        string `json:"revisionId"`
		SerializerVersion string `json:"serializerVersion"`
	} `json:"meta"`
}

type matterRevertRequest struct {
	MatterID  string `json:"matterId"`
	ParentSHA string `json:"parentSHA"`
}

// safeMatterID: single path segment, no separators, no "." / "..".
// SetMattersDirForTest overrides the MattersDir at runtime. Call only from tests.
func (a *API) SetMattersDirForTest(dir string) { a.cfg.OpenCode.MattersDir = dir }

func safeMatterID(s string) bool {
	return s != "" && s != "." && s != ".." && !strings.ContainsAny(s, "/\\\x00")
}

// matterAuth runs the shared access/auth gate. Returns nil user on failure
// (response already written).
func (a *API) matterAuth(w http.ResponseWriter, r *http.Request, scope models.TokenScope) (*models.Document, *models.User) {
	doc, accErr := a.checkDocAccess(r, mux.Vars(r)["id"])
	if accErr != nil {
		a.writeAccessError(w, r, accErr)
		return nil, nil
	}
	user := a.currentUser(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "sign in required")
		return nil, nil
	}
	if !a.enforceScope(w, r, scope) {
		return nil, nil
	}
	if a.cfg.OpenCode.MattersDir == "" {
		writeError(w, http.StatusNotImplemented, "matters dir is not configured")
		return nil, nil
	}
	return doc, user
}

// existingRepo opens the matter repo without creating it.
func (a *API) existingRepo(w http.ResponseWriter, matterID string) *matterrepo.Repo {
	if !safeMatterID(matterID) {
		writeError(w, http.StatusBadRequest, "invalid matterId")
		return nil
	}
	root := filepath.Join(a.cfg.OpenCode.MattersDir, matterID)
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		writeError(w, http.StatusNotFound, "matter repository not found")
		return nil
	}
	repo, err := matterrepo.Open(root)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "open matter repository failed")
		return nil
	}
	return repo
}

func matterRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, matterrepo.ErrStaleParent):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, matterrepo.ErrBadPath), errors.Is(err, matterrepo.ErrBadSHA):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "matter repository error")
	}
}

func (a *API) matterRevisionCommit(w http.ResponseWriter, r *http.Request) {
	doc, user := a.matterAuth(w, r, models.TokenScopeWrite)
	if user == nil {
		return
	}
	capBody(w, r, maxBodyDefault)
	var req matterRevisionRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !safeMatterID(req.MatterID) {
		writeError(w, http.StatusBadRequest, "invalid matterId")
		return
	}
	repo, err := matterrepo.Init(filepath.Join(a.cfg.OpenCode.MattersDir, req.MatterID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "open matter repository failed")
		return
	}
	files := make(map[string][]byte, len(req.Files))
	for p, c := range req.Files {
		files[p] = []byte(c)
	}
	sha, err := repo.Commit(doc.ID, req.ParentSHA, files, matterrepo.CommitMeta{
		Matter:            req.MatterID,
		Document:          doc.ID,
		Actor:             string(models.ActorHuman) + ":" + user.ID,
		Session:           req.Meta.Session,
		Operation:         req.Meta.Operation,
		RevisionID:        req.Meta.RevisionID,
		SerializerVersion: req.Meta.SerializerVersion,
	})
	if err != nil {
		matterRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"sha": sha})
}

func (a *API) matterRevisionDiff(w http.ResponseWriter, r *http.Request) {
	if _, user := a.matterAuth(w, r, models.TokenScopeRead); user == nil {
		return
	}
	q := r.URL.Query()
	repo := a.existingRepo(w, q.Get("matterId"))
	if repo == nil {
		return
	}
	diff, err := repo.Diff(q.Get("from"), q.Get("to"), q.Get("path"))
	if err != nil {
		matterRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"diff": diff})
}

func (a *API) matterRevisionRevert(w http.ResponseWriter, r *http.Request) {
	doc, user := a.matterAuth(w, r, models.TokenScopeWrite)
	if user == nil {
		return
	}
	capBody(w, r, maxBodyDefault)
	var req matterRevertRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	repo := a.existingRepo(w, req.MatterID)
	if repo == nil {
		return
	}
	sha, err := repo.Revert(doc.ID, req.ParentSHA, mux.Vars(r)["sha"], matterrepo.CommitMeta{
		Matter:   req.MatterID,
		Document: doc.ID,
		Actor:    string(models.ActorHuman) + ":" + user.ID,
	})
	if err != nil {
		matterRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"sha": sha})
}
