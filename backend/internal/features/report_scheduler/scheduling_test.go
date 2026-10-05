package report_scheduler

import (
	"testing"
	"time"
)

func TestCalculateNextRunMonthlyUsesConfiguredTimezone(t *testing.T) {
	after := time.Date(2026, time.October, 5, 7, 0, 0, 0, time.UTC) // 10:00 Kampala
	next, err := CalculateNextRun("monthly", "Africa/Kampala", ScheduleTiming{TimeOfDay: "08:00", DayOfMonth: 6}, after)
	if err != nil { t.Fatal(err) }
	want := time.Date(2026, time.October, 6, 5, 0, 0, 0, time.UTC)
	if !next.Equal(want) { t.Fatalf("expected %s, got %s", want, next) }
}

func TestCalculateNextRunWeekly(t *testing.T) {
	after := time.Date(2026, time.October, 5, 9, 0, 0, 0, time.UTC) // Monday 12:00 Kampala
	next, err := CalculateNextRun("weekly", "Africa/Kampala", ScheduleTiming{TimeOfDay: "08:00", Weekday: 1}, after)
	if err != nil { t.Fatal(err) }
	want := time.Date(2026, time.October, 12, 5, 0, 0, 0, time.UTC)
	if !next.Equal(want) { t.Fatalf("expected %s, got %s", want, next) }
}

func TestResolvePreviousMonth(t *testing.T) {
	reference := time.Date(2026, time.October, 5, 9, 0, 0, 0, time.UTC)
	period, err := ResolveReportingPeriod("previous_month", "Africa/Kampala", reference)
	if err != nil { t.Fatal(err) }
	wantStart := time.Date(2026, time.August, 31, 21, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, time.September, 30, 20, 59, 59, 999999999, time.UTC)
	if !period.Start.Equal(wantStart) || !period.End.Equal(wantEnd) {
		t.Fatalf("unexpected period: %+v", period)
	}
}

func TestResolvePreviousEpiWeekStartsMonday(t *testing.T) {
	reference := time.Date(2026, time.October, 5, 9, 0, 0, 0, time.UTC)
	period, err := ResolveReportingPeriod("previous_epi_week", "Africa/Kampala", reference)
	if err != nil { t.Fatal(err) }
	location, _ := time.LoadLocation("Africa/Kampala")
	if period.Start.In(location).Weekday() != time.Monday { t.Fatalf("expected Monday start, got %s", period.Start.In(location).Weekday()) }
}

func TestReportRetryDelay(t *testing.T) {
	cases := []struct {
		attempts int
		want time.Duration
	}{
		{attempts: 1, want: 30 * time.Second},
		{attempts: 2, want: 2 * time.Minute},
		{attempts: 3, want: 5 * time.Minute},
	}
	for _, tc := range cases {
		if got := reportRetryDelay(tc.attempts); got != tc.want {
			t.Fatalf("attempt %d: expected %s, got %s", tc.attempts, tc.want, got)
		}
	}
}
