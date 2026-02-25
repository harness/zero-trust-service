package audit

import "testing"

func TestDateFromEpochMs(t *testing.T) {
	tests := []struct {
		name    string
		epochMs int64
		want    string
	}{
		{"epoch zero", 0, "1970-01-01"},
		{"2024-01-15 noon UTC", 1705320000000, "2024-01-15"},
		{"2026-02-17", 1771286400000, "2026-02-17"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DateFromEpochMs(tc.epochMs)
			if got != tc.want {
				t.Errorf("DateFromEpochMs(%d) = %q, want %q", tc.epochMs, got, tc.want)
			}
		})
	}
}
