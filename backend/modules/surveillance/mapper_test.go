package surveillance

import (
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
	"testing"
	"time"
)

func TestEpiWeekResponsePreservesRequiredDates(t *testing.T) {
	start := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 6)
	response := toEpiWeekResponse(db.EpiWeek{WeekStartDate: start, WeekEndDate: end})
	if response.WeekStartDate == nil || response.WeekEndDate == nil || !response.WeekStartDate.Equal(start) || !response.WeekEndDate.Equal(end) {
		t.Fatalf("reporting period dates were lost in the API response: %+v", response)
	}
}
