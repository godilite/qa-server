package service

import (
	"time"
)

func IsWeeklyAggregation(start, end time.Time) bool {
	return isWeeklyAggregation(start, end)
}
