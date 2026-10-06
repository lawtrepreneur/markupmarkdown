package telemetry

import (
	"context"
	"fmt"
	"time"

	"markupmarkdown/internal/store"
)

type Event struct {
	SuggestionID string    `bson:"suggestion_id" json:"suggestionId"`
	CommentID    string    `bson:"comment_id" json:"commentId"`
	MatterID     string    `bson:"matter_id" json:"matterId"`
	Model        string    `bson:"model" json:"model"`
	Preset       string    `bson:"preset" json:"preset"`
	Outcome      string    `bson:"outcome" json:"outcome"`
	LatencyMs    int64     `bson:"latency_ms" json:"latencyMs"`
	CreatedAt    time.Time `bson:"created_at" json:"createdAt"`
}

func (e Event) Validate() error {
	switch e.Outcome {
	case "pending", "accepted", "rejected", "edited_then_accepted":
	default:
		return fmt.Errorf("invalid telemetry outcome %q", e.Outcome)
	}
	if e.CreatedAt.IsZero() {
		return fmt.Errorf("created_at is required")
	}
	return nil
}

func Record(ctx context.Context, st *store.Store, e Event) error {
	if err := e.Validate(); err != nil {
		return err
	}
	_, err := st.TelemetryEvents().InsertOne(ctx, e)
	return err
}
