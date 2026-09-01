package database

import (
	"testing"
	"time"
)

func TestDecodeAnalyticsSummaryCountsEventsAndFindsLatestTimestamp(t *testing.T) {
	got, err := decodeAnalyticsSummary([]byte(`[
		{"timestamp":"2026-09-01T14:20:00Z","ip":"192.0.2.1"},
		{"timestamp":"2026-09-02T10:00:00Z","user_agent":"Example"}
	]`))
	if err != nil {
		t.Fatalf("decode analytics: %v", err)
	}
	if got.TotalClicks != 2 {
		t.Fatalf("total clicks = %d, want 2", got.TotalClicks)
	}
	if got.LastClickedAt == nil {
		t.Fatal("last clicked at = nil, want timestamp")
	}
	if got.LastClickedAt.Format(time.RFC3339) != "2026-09-02T10:00:00Z" {
		t.Fatalf("last clicked at = %s, want 2026-09-02T10:00:00Z", got.LastClickedAt.Format(time.RFC3339))
	}
}

func TestDecodeAnalyticsSummaryReturnsEmptyAggregateForNoEvents(t *testing.T) {
	got, err := decodeAnalyticsSummary([]byte(`[]`))
	if err != nil {
		t.Fatalf("decode analytics: %v", err)
	}
	if got.TotalClicks != 0 {
		t.Fatalf("total clicks = %d, want 0", got.TotalClicks)
	}
	if got.LastClickedAt != nil {
		t.Fatalf("last clicked at = %v, want nil", got.LastClickedAt)
	}
}
