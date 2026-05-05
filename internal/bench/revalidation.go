package bench

import "strings"

const (
	ConsumerCategoryValid                    = "valid"
	ConsumerCategoryProducerGateFailed       = "producer_gate_failed"
	ConsumerCategoryOTELExportNotSuccessful  = "otel_export_not_success"
	ConsumerCategoryOTELMissingRequiredLabel = "otel_missing_required_label"
	ConsumerCategoryNegativeRequiredMetric   = "otel_negative_required_metric"
)

func RevalidateRunForConsumer(m *RunMetrics) {
	if m.Metadata == nil {
		m.Metadata = map[string]string{}
	}
	reasons := make([]string, 0, 8)
	category := ConsumerCategoryValid

	if m.Metadata["producer_validity_passed"] != "true" {
		category = ConsumerCategoryProducerGateFailed
		reasons = append(reasons, "producer_validity_passed!=true")
	}
	if m.Metadata["otel_export_status"] != "exported" {
		if category == ConsumerCategoryValid {
			category = ConsumerCategoryOTELExportNotSuccessful
		}
		reasons = append(reasons, "otel_export_status!=exported")
	}
	for _, label := range []string{"run_id", "approach", "tier", "seed", "endpoint"} {
		if strings.TrimSpace(m.Metadata["otel_label_"+label]) == "" {
			if category == ConsumerCategoryValid {
				category = ConsumerCategoryOTELMissingRequiredLabel
			}
			reasons = append(reasons, "missing_label_"+label)
		}
	}
	if m.Metadata["otel_label_matrix_id"] == "" {
		// matrix_id remains optional, so no rejection.
	}

	for name, value := range rawMetricValues(*m) {
		if value < 0 {
			if category == ConsumerCategoryValid {
				category = ConsumerCategoryNegativeRequiredMetric
			}
			reasons = append(reasons, "negative_metric_"+name)
		}
	}

	passed := len(reasons) == 0
	m.Metadata["consumer_validity_passed"] = boolString(passed)
	m.Metadata["consumer_rejection_category"] = category
	m.Metadata["consumer_rejection_reasons"] = strings.Join(reasons, ",")
	if !passed {
		m.Metadata["rank_eligible"] = "false"
	}
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
