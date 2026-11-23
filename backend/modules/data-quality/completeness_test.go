package dataquality

import (
	"testing"
	"time"
)

func TestParseCompletenessPeriod(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, eatZone)

	tests := []struct {
		name    string
		raw     string
		now     time.Time
		want    string
		wantErr bool
	}{
		{name: "defaults to last month", raw: "", now: now, want: "2026-08-01"},
		{name: "ended month", raw: "2026-03", now: now, want: "2026-03-01"},
		{name: "trims whitespace", raw: " 2026-08 ", now: now, want: "2026-08-01"},
		{name: "current month is still in progress", raw: "2026-09", now: now, wantErr: true},
		{name: "future month", raw: "2026-10", now: now, wantErr: true},
		{name: "bad format", raw: "08-2026", now: now, wantErr: true},
		{name: "full date rejected", raw: "2026-08-01", now: now, wantErr: true},
		{
			// 22:30 UTC on 30 Sep is already 1 Oct in Kampala, so September has ended.
			name: "month boundary follows EAT not UTC",
			raw:  "2026-09",
			now:  time.Date(2026, 9, 30, 22, 30, 0, 0, time.UTC),
			want: "2026-09-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCompletenessPeriod(tt.raw, tt.now)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %s", got.Format("2006-01-02"))
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Format("2006-01-02") != tt.want {
				t.Fatalf("got %s, want %s", got.Format("2006-01-02"), tt.want)
			}
		})
	}
}
