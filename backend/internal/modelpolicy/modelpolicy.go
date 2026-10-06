// Package modelpolicy loads the flat JSON allowlist of AI models.
package modelpolicy

import (
	"encoding/json"
	"fmt"
	"os"
)

const MaxModels = 10

type Model struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	External bool   `json:"external"`
	Enabled  bool   `json:"enabled"`
}

type Policy struct {
	Models []Model `json:"models"`
}

// Load reads and validates a policy file: {"models":[{...}]}.
func Load(path string) (*Policy, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Policy
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("modelpolicy: %w", err)
	}
	if len(p.Models) > MaxModels {
		return nil, fmt.Errorf("modelpolicy: %d models exceeds max %d", len(p.Models), MaxModels)
	}
	seen := map[string]bool{}
	for _, m := range p.Models {
		if m.ID == "" {
			return nil, fmt.Errorf("modelpolicy: empty model id")
		}
		if seen[m.ID] {
			return nil, fmt.Errorf("modelpolicy: duplicate model id %q", m.ID)
		}
		seen[m.ID] = true
	}
	return &p, nil
}

// Enabled returns enabled models.
func (p *Policy) Enabled() []Model {
	var out []Model
	for _, m := range p.Models {
		if m.Enabled {
			out = append(out, m)
		}
	}
	return out
}

// Allowed reports whether id is an enabled model.
func (p *Policy) Allowed(id string) bool {
	for _, m := range p.Models {
		if m.ID == id {
			return m.Enabled
		}
	}
	return false
}
