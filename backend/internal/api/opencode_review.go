package api

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"

	"markupmarkdown/internal/models"
)

type opencodeReviewRequest struct {
	MatterID      string   `json:"matterId"`
	SelectedPaths []string `json:"selectedPaths"`
	Preset        string   `json:"preset"`
	Model         string   `json:"model"`
}

func (a *API) opencodeReview(w http.ResponseWriter, r *http.Request) {
	doc, accErr := a.checkDocAccess(r, mux.Vars(r)["id"])
	if accErr != nil {
		a.writeAccessError(w, r, accErr)
		return
	}
	user := a.currentUser(r)
	if user == nil || !a.enforceScope(w, r, models.TokenScopeWrite) {
		if user == nil {
			writeError(w, http.StatusUnauthorized, "sign in required")
		}
		return
	}
	if a.oc == nil || a.cfg.OpenCode.MattersDir == "" {
		writeError(w, http.StatusNotImplemented, "OpenCode review is not configured")
		return
	}
	capBody(w, r, maxBodyDefault)
	var req opencodeReviewRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.MatterID == "" || req.Preset == "" {
		writeError(w, http.StatusBadRequest, "matterId and preset are required")
		return
	}
	result, err := a.oc.RunReview(r.Context(), req.MatterID,
		filepath.Join(a.cfg.OpenCode.MattersDir, req.MatterID), req.SelectedPaths,
		req.Preset, req.Model, doc.Content)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	info, _ := tokenInfoFromRequest(r)
	comments := make([]*models.Comment, 0, len(result.Suggestions))
	for _, suggestion := range result.Suggestions {
		comment, err := a.AddSuggestion(r.Context(), user.ID, doc.ID, suggestion.Rationale,
			suggestion.Quoted, 1, suggestion.Replacement, info.TokenID, info.Label)
		if err != nil {
			internalError(w, "opencode_review.comment", err)
			return
		}
		comments = append(comments, comment)
	}
	a.hub.Broadcast(doc.ID, "comments-updated")
	writeJSON(w, http.StatusCreated, map[string]any{
		"comments":  comments,
		"dropped":   result.Dropped,
		"ambiguous": result.Ambiguous,
		"manifest": map[string]any{
			"matterId":      result.Manifest.MatterID(),
			"sessionId":     result.Manifest.SessionID(),
			"selectedPaths": result.Manifest.SelectedPaths(),
			"readFiles":     result.Manifest.ReadFiles(),
			"readTracking":  result.Manifest.ReadTracking(),
			"createdAt":     result.Manifest.CreatedAt().Format(time.RFC3339Nano),
		},
	})
}
