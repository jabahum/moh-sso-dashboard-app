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

func TestScheduleWithinHealthScope(t *testing.T) {
	schedule := Schedule{HealthContext: HealthContext{District: "Kampala", Facility: "Kisenyi HC IV"}}
	if !scheduleWithinHealthScope(schedule, HealthContext{District: "Kampala"}) { t.Fatal("expected district scope to match") }
	if scheduleWithinHealthScope(schedule, HealthContext{District: "Wakiso"}) { t.Fatal("expected different district to be denied") }
	if !scheduleWithinHealthScope(schedule, HealthContext{District: "Kampala", Facility: "Kisenyi HC IV"}) { t.Fatal("expected facility scope to match") }
	if scheduleWithinHealthScope(schedule, HealthContext{District: "Kampala", Facility: "Kiruddu NRH"}) { t.Fatal("expected different facility to be denied") }
	if !scheduleWithinHealthScope(schedule, HealthContext{}) { t.Fatal("expected unscoped administrator to match") }
}

func TestDeliveryTokenHashIsStableAndDoesNotExposeToken(t *testing.T) {
	token := "sample-delivery-token"
	first := hashDeliveryToken(token)
	second := hashDeliveryToken(token)
	if first != second { t.Fatal("expected delivery token hash to be deterministic") }
	if first == token { t.Fatal("delivery token must not be stored in plaintext") }
	if len(first) != 64 { t.Fatalf("expected SHA-256 hex hash length 64, got %d", len(first)) }
}
