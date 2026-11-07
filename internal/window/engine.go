package window

import (
	"time"

	"e-go05/internal/clock"
)

type Key struct {
	Start   time.Time
	Country string
}

type Result struct {
	Key
	End         time.Time
	Count       int64
	GMVCents    int64
	Revision    int64
	Final       bool
	PublishedAt time.Time
}

type state struct {
	result  Result
	emitted bool
}

type Engine struct {
	clock     clock.Clock
	lateness  time.Duration
	watermark time.Time
	active    map[Key]*state

	AcceptedEvents int64
	LateEvents     int64
	LateCents      int64
	Emissions      int64
	Corrections    int64
	Finalized      int64
	MaxActive      int
}

func NewEngine(c clock.Clock, lateness time.Duration) *Engine {
	return &Engine{
		clock:    c,
		lateness: lateness,
		active:   make(map[Key]*state),
	}
}

// Advance emits and finalizes windows before processing the next event.
func (e *Engine) Advance(next time.Time) []Result {
	if !next.After(e.watermark) {
		return nil
	}

	e.watermark = next
	var updates []Result

	for key, s := range e.active {
		if !s.emitted && !next.Before(s.result.End) {
			s.emitted = true
			s.result.Revision++
			s.result.PublishedAt = e.clock.Now()
			e.Emissions++
			updates = append(updates, s.result)
		}

		deadline := s.result.End.Add(e.lateness)
		if !next.Before(deadline) {
			s.result.Final = true
			s.result.Revision++
			s.result.PublishedAt = e.clock.Now()
			e.Finalized++
			updates = append(updates, s.result)

			delete(e.active, key)
		}
	}

	return updates
}

func (e *Engine) Add(
	eventTime time.Time,
	country string,
	cents int64,
) []Result {
	start, end := Bounds(eventTime)

	// This check also prevents recreating an already finalized window.
	if !e.watermark.IsZero() &&
		!e.watermark.Before(end.Add(e.lateness)) {
		e.LateEvents++
		e.LateCents += cents
		return nil
	}

	key := Key{Start: start, Country: country}
	s, exists := e.active[key]

	if !exists {
		s = &state{
			result: Result{
				Key: key,
				End: end,
			},
		}
		e.active[key] = s

		if len(e.active) > e.MaxActive {
			e.MaxActive = len(e.active)
		}
	}

	s.result.Count++
	s.result.GMVCents += cents
	e.AcceptedEvents++

	// An already emitted window receives a correction.
	if s.emitted {
		s.result.Revision++
		s.result.PublishedAt = e.clock.Now()
		e.Corrections++
		return []Result{s.result}
	}

	// The first event for this country-window may itself arrive late.
	if !e.watermark.IsZero() && !e.watermark.Before(end) {
		s.emitted = true
		s.result.Revision++
		s.result.PublishedAt = e.clock.Now()
		e.Emissions++
		return []Result{s.result}
	}

	return nil
}

func (e *Engine) ActiveCount() int {
	return len(e.active)
}

// Finish finalizes remaining windows once the finite input is exhausted.
func (e *Engine) Finish() []Result {
	if len(e.active) == 0 {
		return nil
	}

	lastDeadline := e.watermark

	for _, s := range e.active {
		deadline := s.result.End.Add(e.lateness)

		if deadline.After(lastDeadline) {
			lastDeadline = deadline
		}
	}

	return e.Advance(lastDeadline)
}
