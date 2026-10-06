package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const ReviewTimeout = 10 * time.Minute

type Suggestion struct {
	Quoted      string `json:"quoted"`
	Replacement string `json:"replacement"`
	Rationale   string `json:"rationale"`
}

// Presets are versioned prompt templates; key = preset name.
var Presets = map[string]string{
	"verify_citations": "v1: Verify each citation in the document against the selected files. ",
	"flag_unsupported": "v1: Flag claims in the document not supported by the selected files. ",
	"tighten_prose":    "v1: Tighten wording of the document without changing meaning. ",
}

const outputSpec = `Reply ONLY with a JSON array of {"quoted","replacement","rationale"}; "quoted" must be copied verbatim from the document.`

type Result struct {
	Manifest    Manifest
	Suggestions []Suggestion
	Dropped     int // quoted not found in doc
	Ambiguous   int // quoted found more than once in doc
}

// RunReview runs one review bounded by ReviewTimeout.
func (c *Client) RunReview(ctx context.Context, matterID, matterRoot string, selectedPaths []string, preset, model, doc string) (*Result, error) {
	return c.runReview(ctx, ReviewTimeout, matterID, matterRoot, selectedPaths, preset, model, doc)
}

func (c *Client) runReview(ctx context.Context, timeout time.Duration, matterID, matterRoot string, selectedPaths []string, preset, model, doc string) (*Result, error) {
	tpl, ok := Presets[preset]
	if !ok {
		return nil, fmt.Errorf("opencode: unknown preset %q", preset)
	}
	if err := validatePaths(matterRoot, selectedPaths); err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = ReviewTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	sid, err := c.CreateSession(ctx, matterRoot)
	if err != nil {
		return nil, err
	}
	defer func() {
		// fresh ctx: request ctx may already be expired
		cc, cf := context.WithTimeout(context.Background(), 10*time.Second)
		defer cf()
		_ = c.CloseSession(cc, sid)
	}()
	m := newManifest(matterID, sid, selectedPaths)
	prompt := tpl + "Files: " + strings.Join(selectedPaths, ", ") + "\n" + outputSpec + "\n\nDOCUMENT:\n" + doc
	out, err := c.send(ctx, sid, model, prompt)
	if err != nil {
		return nil, err
	}
	all, err := parseSuggestions(out)
	if err != nil {
		return nil, err
	}
	res := &Result{Manifest: m}
	for _, s := range all {
		n := 0
		if s.Quoted != "" {
			n = strings.Count(doc, s.Quoted)
		}
		switch n {
		case 1:
			res.Suggestions = append(res.Suggestions, s)
		case 0:
			res.Dropped++
		default:
			res.Ambiguous++
		}
	}
	return res, nil
}

func parseSuggestions(s string) ([]Suggestion, error) {
	s = strings.TrimSpace(s)
	if i, j := strings.Index(s, "["), strings.LastIndex(s, "]"); i >= 0 && j > i {
		s = s[i : j+1]
	}
	var out []Suggestion
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("opencode: parse suggestions: %w", err)
	}
	return out, nil
}
