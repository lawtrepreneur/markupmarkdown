package telemetry_test

// Tests for telemetry.Event validation and telemetry.Record persistence.
// The Validate tests are pure unit tests; Record is an integration test
// gated on the shared test database (skips itself without MONGODB_URI).

import (
	"context"
	"testing"
	"time"

	"markupmarkdown/internal/telemetry"
	"markupmarkdown/internal/testutil"
)

func TestValidate_AcceptsAllKnownOutcomes(t *testing.T) {
	for _, outcome := range []string{
		"pending", "accepted", "rejected", "edited_then_accepted",
	} {
		e := telemetry.Event{Outcome: outcome, CreatedAt: time.Now().UTC()}
		if err := e.Validate(); err != nil {
			t.Errorf("outcome %q: %v", outcome, err)
		}
	}
}

func TestValidate_RejectsUnknownOutcome(t *testing.T) {
	e := telemetry.Event{Outcome: "meh", CreatedAt: time.Now().UTC()}
	if err := e.Validate(); err == nil {
		t.Error("unknown outcome accepted")
	}
}

func TestValidate_RequiresCreatedAt(t *testing.T) {
	e := telemetry.Event{Outcome: "accepted"}
	if err := e.Validate(); err == nil {
		t.Error("zero created_at accepted")
	}
}

func TestRecord_PersistsEvent(t *testing.T) {
	st, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	e := telemetry.Event{
		SuggestionID: "s1", CommentID: "c1", MatterID: "m1",
		Model: "suggestion", Outcome: "accepted", CreatedAt: time.Now().UTC(),
	}
	if err := telemetry.Record(context.Background(), st, e); err != nil {
		t.Fatalf("record: %v", err)
	}
	n, err := st.TelemetryEvents().CountDocuments(context.Background(),
		map[string]any{"suggestion_id": "s1"})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("count=%d want 1", n)
	}
}

func TestRecord_RejectsInvalidEvent(t *testing.T) {
	st, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	err := telemetry.Record(context.Background(), st,
		telemetry.Event{Outcome: "bogus", CreatedAt: time.Now().UTC()})
	if err == nil {
		t.Fatal("invalid event recorded")
	}
	n, _ := st.TelemetryEvents().CountDocuments(context.Background(), map[string]any{})
	if n != 0 {
		t.Fatalf("count=%d want 0", n)
	}
}
