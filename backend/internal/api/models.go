package api

import (
	"net/http"

	"markupmarkdown/internal/modelpolicy"
)

type modelResponse struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	External bool   `json:"external"`
}

func (a *API) listModels(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	a.writeModels(w, a.cfg.ModelPolicyPath)
}

func (a *API) writeModels(w http.ResponseWriter, path string) {
	if path == "" {
		writeJSON(w, http.StatusOK, []modelResponse{})
		return
	}
	p, err := modelpolicy.Load(path)
	if err != nil {
		internalError(w, "listModels", err)
		return
	}
	enabled := p.Enabled()
	out := make([]modelResponse, len(enabled))
	for i, m := range enabled {
		out[i] = modelResponse{ID: m.ID, Provider: m.Provider, External: m.External}
	}
	writeJSON(w, http.StatusOK, out)
}
