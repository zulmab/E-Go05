package window

import "time"

type Watermark struct {
	delay        time.Duration
	maxEventTime time.Time
	current      time.Time
}

func NewWatermark(delay time.Duration) *Watermark {
	return &Watermark{delay: delay}
}

func (w *Watermark) Observe(eventTime time.Time) {
	if w.maxEventTime.IsZero() || eventTime.After(w.maxEventTime) {
		w.maxEventTime = eventTime
		w.current = eventTime.Add(-w.delay)
	}
}

func (w *Watermark) Value() time.Time {
	return w.current
}

// ObserveIfValid rejects suspicious timestamps before updating the watermark.
func (w *Watermark) ObserveIfValid(
	eventTime time.Time,
	arrivalTime time.Time,
	futureTolerance time.Duration,
) bool {
	if eventTime.After(arrivalTime.Add(futureTolerance)) {
		return false
	}

	w.Observe(eventTime)
	return true
}
