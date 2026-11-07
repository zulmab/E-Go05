package window

import "time"

const Duration = 5 * time.Minute

func Bounds(eventTime time.Time) (time.Time, time.Time) {
	start := eventTime.UTC().Truncate(Duration)
	end := start.Add(Duration)

	return start, end
}
