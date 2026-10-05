package report_scheduler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func normalizeTiming(input ScheduleTiming) (ScheduleTiming, error) {
	input.TimeOfDay = strings.TrimSpace(input.TimeOfDay)
	if input.TimeOfDay == "" {
		input.TimeOfDay = "08:00"
	}
	parts := strings.Split(input.TimeOfDay, ":")
	if len(parts) != 2 {
		return input, errors.New("timeOfDay must use HH:MM format")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return input, errors.New("timeOfDay hour must be between 00 and 23")
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return input, errors.New("timeOfDay minute must be between 00 and 59")
	}
	if input.Weekday < 0 || input.Weekday > 7 {
		return input, errors.New("weekday must be between 1 (Monday) and 7 (Sunday)")
	}
	if input.DayOfMonth < 0 || input.DayOfMonth > 31 {
		return input, errors.New("dayOfMonth must be between 1 and 31")
	}
	return input, nil
}

func CalculateNextRun(frequency, timezone string, timing ScheduleTiming, after time.Time) (time.Time, error) {
	timing, err := normalizeTiming(timing)
	if err != nil {
		return time.Time{}, err
	}
	location, err := time.LoadLocation(strings.TrimSpace(timezone))
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timezone %q: %w", timezone, err)
	}
	parts := strings.Split(timing.TimeOfDay, ":")
	hour, _ := strconv.Atoi(parts[0])
	minute, _ := strconv.Atoi(parts[1])
	localAfter := after.In(location)
	frequency = strings.ToLower(strings.TrimSpace(frequency))

	makeCandidate := func(year int, month time.Month, day int) time.Time {
		lastDay := time.Date(year, month+1, 0, hour, minute, 0, 0, location).Day()
		if day > lastDay {
			day = lastDay
	}
		return time.Date(year, month, day, hour, minute, 0, 0, location)
	}

	switch frequency {
	case "daily":
		candidate := makeCandidate(localAfter.Year(), localAfter.Month(), localAfter.Day())
		if !candidate.After(localAfter) {
			candidate = candidate.AddDate(0, 0, 1)
		}
		return candidate.UTC(), nil
	case "weekly":
		weekday := timing.Weekday
		if weekday == 0 { weekday = 1 }
		if weekday == 7 { weekday = int(time.Sunday) }
		delta := (weekday - int(localAfter.Weekday()) + 7) % 7
		candidate := makeCandidate(localAfter.Year(), localAfter.Month(), localAfter.Day()).AddDate(0, 0, delta)
		if !candidate.After(localAfter) {
			candidate = candidate.AddDate(0, 0, 7)
		}
		return candidate.UTC(), nil
	case "monthly":
		day := timing.DayOfMonth
		if day == 0 {
			day = 1
		}
		candidate := makeCandidate(localAfter.Year(), localAfter.Month(), day)
		if !candidate.After(localAfter) {
			next := localAfter.AddDate(0, 1, 0)
			candidate = makeCandidate(next.Year(), next.Month(), day)
		}
		return candidate.UTC(), nil
	case "quarterly":
		day := timing.DayOfMonth
		if day == 0 {
			day = 1
		}
		quarterStartMonth := time.Month(((int(localAfter.Month())-1)/3)*3 + 1)
		candidate := makeCandidate(localAfter.Year(), quarterStartMonth, day)
		if !candidate.After(localAfter) {
			next := candidate.AddDate(0, 3, 0)
			candidate = makeCandidate(next.Year(), next.Month(), day)
		}
		return candidate.UTC(), nil
	case "annual", "yearly":
		day := timing.DayOfMonth
		if day == 0 {
			day = 1
		}
		candidate := makeCandidate(localAfter.Year(), time.January, day)
		if !candidate.After(localAfter) {
			candidate = makeCandidate(localAfter.Year()+1, time.January, day)
		}
		return candidate.UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported schedule frequency %q", frequency)
	}
}

func ResolveReportingPeriod(strategy, timezone string, reference time.Time) (ResolvedPeriod, error) {
	location, err := time.LoadLocation(strings.TrimSpace(timezone))
	if err != nil {
		return ResolvedPeriod{}, fmt.Errorf("invalid timezone %q: %w", timezone, err)
	}
	now := reference.In(location)
	dayStart := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, location) }
	dayEnd := func(t time.Time) time.Time { return dayStart(t).AddDate(0, 0, 1).Add(-time.Nanosecond) }
	weekStart := func(t time.Time) time.Time {
		delta := (int(t.Weekday()) - int(time.Monday) + 7) % 7
		return dayStart(t).AddDate(0, 0, -delta)
	}
	monthStart := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, location) }
	quarterStart := func(t time.Time) time.Time {
		month := time.Month(((int(t.Month())-1)/3)*3 + 1)
		return time.Date(t.Year(), month, 1, 0, 0, 0, 0, location)
	}
	yearStart := func(t time.Time) time.Time { return time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, location) }

	var start, end time.Time
	strategy = strings.ToLower(strings.TrimSpace(strategy))
	switch strategy {
	case "current_day":
		start, end = dayStart(now), dayEnd(now)
	case "previous_day":
		previous := now.AddDate(0, 0, -1); start, end = dayStart(previous), dayEnd(previous)
	case "current_week", "current_epi_week":
		start = weekStart(now); end = start.AddDate(0, 0, 7).Add(-time.Nanosecond)
	case "previous_week", "previous_epi_week":
		start = weekStart(now).AddDate(0, 0, -7); end = start.AddDate(0, 0, 7).Add(-time.Nanosecond)
	case "current_month":
		start = monthStart(now); end = start.AddDate(0, 1, 0).Add(-time.Nanosecond)
	case "previous_month":
		start = monthStart(now).AddDate(0, -1, 0); end = start.AddDate(0, 1, 0).Add(-time.Nanosecond)
	case "current_quarter":
		start = quarterStart(now); end = start.AddDate(0, 3, 0).Add(-time.Nanosecond)
	case "previous_quarter":
		start = quarterStart(now).AddDate(0, -3, 0); end = start.AddDate(0, 3, 0).Add(-time.Nanosecond)
	case "current_year":
		start = yearStart(now); end = start.AddDate(1, 0, 0).Add(-time.Nanosecond)
	case "previous_year":
		start = yearStart(now).AddDate(-1, 0, 0); end = start.AddDate(1, 0, 0).Add(-time.Nanosecond)
	default:
		return ResolvedPeriod{}, fmt.Errorf("unsupported reporting period strategy %q", strategy)
	}
	return ResolvedPeriod{Strategy: strategy, Start: start.UTC(), End: end.UTC()}, nil
}

func buildExecutionParameters(schedule Schedule, period ResolvedPeriod) map[string]any {
	out := make(map[string]any, len(schedule.Parameters)+2)
	for key, value := range schedule.Parameters {
		out[key] = value
	}
	reportingPeriod := map[string]any{
		"strategy": period.Strategy,
		"start":    period.Start.Format(time.RFC3339),
		"end":      period.End.Format(time.RFC3339),
	}
	if strings.Contains(period.Strategy, "epi_week") {
		year, week := period.Start.ISOWeek()
		reportingPeriod["epiYear"] = year
		reportingPeriod["epiWeek"] = week
	}
	out["reporting_period"] = reportingPeriod
	out["health_context"] = map[string]any{
		"level":    schedule.HealthContext.Level,
		"district": schedule.HealthContext.District,
		"facility": schedule.HealthContext.Facility,
	}
	return out
}
