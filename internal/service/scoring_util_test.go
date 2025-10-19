package service_test

import (
	"testing"
	"time"

	"github.com/godilite/qa-server/internal/service"
	"github.com/stretchr/testify/assert"
)

func TestIsWeeklyAggregation_Utils(t *testing.T) {
	cases := []struct {
		name     string
		start    time.Time
		end      time.Time
		expected bool
	}{
		{
			name:     "Less than 4 weeks → daily",
			start:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			end:      time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "Exactly 4 weeks → weekly",
			start:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			end:      time.Date(2025, 1, 29, 0, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "Over a calendar month difference → weekly",
			start:    time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			end:      time.Date(2025, 2, 16, 0, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "Spanning February (short month) → weekly",
			start:    time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
			end:      time.Date(2025, 2, 28, 0, 0, 0, 0, time.UTC),
			expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := service.IsWeeklyAggregation(tc.start, tc.end)
			assert.Equal(t, tc.expected, got)
		})
	}
}
