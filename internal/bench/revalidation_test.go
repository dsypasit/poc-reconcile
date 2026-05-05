package bench

import "testing"

func TestRevalidateRunForConsumerRejectsInvalidRun(t *testing.T) {
	m := RunMetrics{
		RunID:      "r1",
		Approach:   "approach-a",
		Tier:       "small",
		Seed:       42,
		DurationMS: -1,
		Errors:     map[string]int{"ingest": 0, "cutover": 0, "cleanup": 0},
		Counts:     map[string]int{"records_written": 1, "records_deleted": 0},
		Metadata: map[string]string{
			"producer_validity_passed": "false",
			"otel_export_status":      "failed",
			"otel_label_run_id":       "r1",
			"otel_label_approach":     "approach-a",
			"otel_label_tier":         "small",
			"otel_label_seed":         "42",
			"otel_label_endpoint":     "",
		},
	}
	RevalidateRunForConsumer(&m)
	if m.Metadata["consumer_validity_passed"] != "false" {
		t.Fatalf("expected consumer_validity_passed=false, got %q", m.Metadata["consumer_validity_passed"])
	}
	if m.Metadata["consumer_rejection_category"] == ConsumerCategoryValid {
		t.Fatalf("expected non-valid rejection category")
	}
	if m.Metadata["rank_eligible"] != "false" {
		t.Fatalf("expected rank_eligible=false, got %q", m.Metadata["rank_eligible"])
	}
}
