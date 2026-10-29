package school

import (
	"fmt"
	"time"
)

func parseTimeString(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}
	if t, err := time.Parse("15:04:05", s); err == nil {
		return t, nil
	}
	return time.Parse("15:04", s)
}

func timeToMinutes(t time.Time) int {
	return t.Hour()*60 + t.Minute()
}