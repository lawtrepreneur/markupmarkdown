package api

import (
	"testing"

	"markupmarkdown/internal/models"
)

func TestIsDocLevel(t *testing.T) {
	cases := []struct {
		name string
		a    models.Anchor
		want bool
	}{
		{"empty", models.Anchor{}, true},
		{"only-whitespace-exact", models.Anchor{Exact: "   "}, true},
		{"with-text", models.Anchor{Start: 0, End: 4, Exact: "abcd"}, false},
		{"start-end-set-empty-exact", models.Anchor{Start: 0, End: 10}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDocLevel(tc.a); got != tc.want {
				t.Errorf("isDocLevel(%+v)=%v want %v", tc.a, got, tc.want)
			}
		})
	}
}

func TestReanchorComments_CleanWhenExactFound(t *testing.T) {
	comments := []models.Comment{{
		ID:     "c1",
		Anchor: models.Anchor{Start: 5, End: 10, Exact: "hello"},
	}}
	out := reanchorComments(comments, "say hello world")
	if out[0].Status != reanchorClean {
		t.Fatalf("status=%v want clean", out[0].Status)
	}
	if out[0].Exact != "hello" {
		t.Fatalf("exact=%q want hello", out[0].Exact)
	}
}

func TestReanchorComments_OrphanWhenExactGone(t *testing.T) {
	comments := []models.Comment{{
		ID:     "c1",
		Anchor: models.Anchor{Start: 5, End: 12, Exact: "goodbye"},
	}}
	out := reanchorComments(comments, "say hello world")
	if out[0].Status != reanchorOrphan {
		t.Fatalf("status=%v want orphan", out[0].Status)
	}
	if out[0].OriginalExact != "goodbye" {
		t.Fatalf("originalExact=%q want goodbye", out[0].OriginalExact)
	}
}

func TestReanchorComments_DocLevelLeftAlone(t *testing.T) {
	comments := []models.Comment{{
		ID:     "c1",
		Anchor: models.Anchor{}, // doc-level: empty
	}}
	out := reanchorComments(comments, "anything")
	if out[0].Status != reanchorDocLevel {
		t.Fatalf("status=%v want docLevel", out[0].Status)
	}
}

func TestReanchorComments_WholesaleRewriteOrphansAnchors(t *testing.T) {
	comments := []models.Comment{
		{ID: "c1", Anchor: models.Anchor{Exact: "first paragraph"}},
		{ID: "c2", Anchor: models.Anchor{Exact: "second paragraph"}},
		{ID: "c3", Anchor: models.Anchor{}},
	}
	out := reanchorComments(comments, "A wholly rewritten document with none of the original text.")
	if out[0].Status != reanchorOrphan || out[0].OriginalExact != "first paragraph" {
		t.Fatalf("first=%+v want orphan with original exact", out[0])
	}
	if out[1].Status != reanchorOrphan || out[1].OriginalExact != "second paragraph" {
		t.Fatalf("second=%+v want orphan with original exact", out[1])
	}
	if out[2].Status != reanchorDocLevel {
		t.Fatalf("doc-level status=%v want doc-level", out[2].Status)
	}
}

func TestReanchorComments_OrphanRevivesWhenSourceRestored(t *testing.T) {
	// Comment was previously marked orphan. anchor.exact is preserved
	// from before. If the user reverts the source and the original text
	// reappears, the next sync un-orphans the comment.
	comments := []models.Comment{{
		ID: "c1",
		Anchor: models.Anchor{
			Start: 5, End: 16,
			Exact: "hello there",
		},
		Orphan:        true,
		OriginalExact: "hello there",
	}}
	out := reanchorComments(comments, "well hello there friend")
	if out[0].Status != reanchorClean {
		t.Fatalf("status=%v want clean", out[0].Status)
	}
	if out[0].Exact != "hello there" {
		t.Fatalf("exact=%q want 'hello there'", out[0].Exact)
	}
}

func TestValidateAnchor_AllowsDocLevel(t *testing.T) {
	if err := ValidateAnchor(models.Anchor{}); err != nil {
		t.Fatalf("doc-level anchor should validate: %v", err)
	}
}

func TestValidateAnchor_RejectsInvalidRange(t *testing.T) {
	if err := ValidateAnchor(models.Anchor{Start: 10, End: 5, Exact: "x"}); err == nil {
		t.Fatal("invalid range should fail")
	}
}

func TestValidateManualAnchor_RequiresExactInContent(t *testing.T) {
	err := validateManualAnchor(patchCommentAnchorRequest{
		Start: 0, End: 5, Exact: "missing",
	}, "this is the doc content")
	if err == nil {
		t.Fatal("expected error when exact is not in content")
	}
}

func TestValidateManualAnchor_AcceptsMatchInContent(t *testing.T) {
	err := validateManualAnchor(patchCommentAnchorRequest{
		Start: 0, End: 5, Exact: "this",
	}, "this is the doc content")
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
}

const anchorSentence = "the verification stage runs the full integration suite against a disposable database"

func reanchorOne(t *testing.T, a models.Anchor, doc string) reanchorResult {
	t.Helper()
	return reanchorComments([]models.Comment{{ID: "c1", Anchor: a}}, doc)[0]
}

func TestReanchor_ParagraphSplit(t *testing.T) {
	doc := "Intro line.\n\nThe pipeline has stages. Then " + anchorSentence + " and reports.\n\nTail."
	split := "Intro line.\n\nThe pipeline has stages.\n\nThen the verification stage runs the full, integration suite against a disposable database and reports.\n\nTail."
	res := reanchorOne(t, models.Anchor{Exact: anchorSentence}, doc)
	if res.Status != reanchorClean || res.Fuzzy {
		t.Fatalf("exact baseline: %+v", res)
	}
	res = reanchorOne(t, models.Anchor{Exact: anchorSentence, ParagraphID: res.ParagraphID}, split)
	if res.Status != reanchorClean || !res.Fuzzy || res.ParagraphID == "" {
		t.Fatalf("split: %+v", res)
	}
}

func TestReanchor_ParagraphMerge(t *testing.T) {
	merged := "Intro line. Then the verification stage runs the full, integration suite against a disposable database. Tail."
	res := reanchorOne(t, models.Anchor{Exact: anchorSentence, ParagraphID: "3"}, merged)
	if res.Status != reanchorClean || !res.Fuzzy {
		t.Fatalf("merge: %+v", res)
	}
}

func TestReanchor_SentenceMoved(t *testing.T) {
	moved := "Moved up top: the verification stage runs the full, integration suite against a disposable database.\n\nOther paragraph about rollbacks being manual."
	res := reanchorOne(t, models.Anchor{Exact: anchorSentence, ParagraphID: "5"}, moved)
	if res.Status != reanchorClean || !res.Fuzzy || res.ParagraphID != "0" {
		t.Fatalf("moved: %+v", res)
	}
}

func TestReanchor_PrefixSuffixDisambiguatesExact(t *testing.T) {
	doc := "alpha one. ship it now. beta.\n\nGamma two. ship it now. delta."
	res := reanchorOne(t, models.Anchor{Exact: "ship it now", Prefix: "Gamma two. ", Suffix: ". delta."}, doc)
	if res.Status != reanchorClean || res.Fuzzy || res.ParagraphID != "1" {
		t.Fatalf("prefix/suffix: %+v", res)
	}
}

func TestReanchor_FuzzyWinsByParagraphID(t *testing.T) {
	doc := "The verification stage runs the full, integration suite against a disposable database.\n\n" +
		"The verification stage runs the full integration suite against a disposable databases."
	res := reanchorOne(t, models.Anchor{Exact: anchorSentence, ParagraphID: "1"}, doc)
	if res.Status != reanchorClean || !res.Fuzzy || res.ParagraphID != "1" {
		t.Fatalf("paragraph winner: %+v", res)
	}
	res = reanchorOne(t, models.Anchor{Exact: anchorSentence}, doc)
	if res.Status != reanchorOrphan {
		t.Fatalf("ambiguous must orphan: %+v", res)
	}
}
