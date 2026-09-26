package bot

import (
	"testing"
	"time"
)

func TestParseReminderTime(t *testing.T) {
	tests := []struct {
		value      string
		wantHour   int
		wantMinute int
		wantErr    bool
	}{
		{value: "09:00", wantHour: 9, wantMinute: 0},
		{value: "18:30", wantHour: 18, wantMinute: 30},
		{value: "23:59", wantHour: 23, wantMinute: 59},
		{value: "9:00", wantErr: true},
		{value: "24:00", wantErr: true},
		{value: "18:60", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			hour, minute, err := parseReminderTime(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("parseReminderTime(%q) error = %v, wantErr %t", test.value, err, test.wantErr)
			}
			if err == nil && (hour != test.wantHour || minute != test.wantMinute) {
				t.Fatalf("parseReminderTime(%q) = %02d:%02d, want %02d:%02d", test.value, hour, minute, test.wantHour, test.wantMinute)
			}
		})
	}
}

func TestParseFlexibleDateInLocationUsesProvidedLocation(t *testing.T) {
	location, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	if err != nil {
		t.Fatalf("load Argentina timezone: %v", err)
	}

	got, err := parseFlexibleDateInLocation("30/09/2026", location)
	if err != nil {
		t.Fatalf("parseFlexibleDateInLocation returned error: %v", err)
	}
	if got.Location() != location {
		t.Fatalf("location = %q, want %q", got.Location(), location)
	}
	if got.Year() != 2026 || got.Month() != time.September || got.Day() != 30 {
		t.Fatalf("date = %s, want 30/09/2026", got.Format("02/01/2006"))
	}
}
