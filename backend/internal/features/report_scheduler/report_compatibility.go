package report_scheduler

import (
	"encoding/json"
	"fmt"
)

// Older providers may omit period metadata; an explicit empty list permits no periods.
func validateReportPeriod(report HealthBIReport, strategy, timezone string) error {
	raw, advertised := report.Metadata["periodStrategies"]
	if !advertised {
		return nil
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("invalid Health BI reporting period metadata")
	}
	var supported []string
	if err := json.Unmarshal(payload, &supported); err != nil {
		return fmt.Errorf("invalid Health BI reporting period metadata")
	}
	if timezone != "Africa/Kampala" {
		return fmt.Errorf("Health BI reporting periods require Africa/Kampala timezone")
	}
	for _, period := range supported {
		if period == strategy {
			return nil
		}
	}
	return fmt.Errorf("reporting period %q is not supported by Health BI report %q", strategy, report.Name)
}
