package main

import (
	"testing"
	"time"
)

func TestSameUTCDay(t *testing.T) {
	beijing := time.FixedZone("CST", 8*60*60)

	tests := []struct {
		name string
		a, b time.Time
		want bool
	}{
		{
			// The three cron attempts of one day, as the workflow fires them.
			name: "two attempts on the same UTC day",
			a:    time.Date(2026, 8, 27, 2, 11, 0, 0, time.UTC),
			b:    time.Date(2026, 8, 27, 6, 17, 0, 0, time.UTC),
			want: true,
		},
		{
			name: "consecutive days",
			a:    time.Date(2026, 8, 26, 23, 59, 59, 0, time.UTC),
			b:    time.Date(2026, 8, 27, 0, 0, 1, 0, time.UTC),
			want: false,
		},
		{
			// 2026-08-27 06:00 UTC is still the 27th in Beijing, but the
			// comparison must not be swayed by either value's location.
			name: "same instant expressed in different zones",
			a:    time.Date(2026, 8, 27, 14, 0, 0, 0, beijing),
			b:    time.Date(2026, 8, 27, 6, 0, 0, 0, time.UTC),
			want: true,
		},
		{
			// The trap this function exists to avoid: 08-28 01:00 Beijing is
			// still 08-27 in UTC, so a local-date comparison would call these
			// different days and regenerate a digest that already exists.
			name: "Beijing has rolled over but UTC has not",
			a:    time.Date(2026, 8, 28, 1, 0, 0, 0, beijing),
			b:    time.Date(2026, 8, 27, 20, 0, 0, 0, time.UTC),
			want: true,
		},
		{
			// A state.json that failed to parse leaves the zero time; it must
			// never look like today, or the digest would never regenerate.
			name: "zero time is never today",
			a:    time.Time{},
			b:    time.Date(2026, 8, 27, 2, 11, 0, 0, time.UTC),
			want: false,
		},
		{
			name: "same day of month a year apart",
			a:    time.Date(2025, 8, 27, 2, 11, 0, 0, time.UTC),
			b:    time.Date(2026, 8, 27, 2, 11, 0, 0, time.UTC),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameUTCDay(tt.a, tt.b); got != tt.want {
				t.Errorf("sameUTCDay(%s, %s) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
			if got := sameUTCDay(tt.b, tt.a); got != tt.want {
				t.Errorf("sameUTCDay(%s, %s) = %v, want %v (not symmetric)", tt.b, tt.a, got, tt.want)
			}
		})
	}
}
