package tests

import (
	"testing"
	"time"

	"e-go05/internal/clock"
	"e-go05/internal/window"
)

func baseTime() time.Time {
	return time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
}

func TestWindowBoundaries(t *testing.T) {
	base := baseTime()

	cases := []struct {
		name  string
		event time.Time
		start time.Time
	}{
		{"start included", base, base},
		{
			"just before end",
			base.Add(5*time.Minute - time.Nanosecond),
			base,
		},
		{
			"end belongs to next window",
			base.Add(5 * time.Minute),
			base.Add(5 * time.Minute),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := window.Bounds(tc.event)

			if !start.Equal(tc.start) {
				t.Fatalf("start: got %s, want %s", start, tc.start)
			}

			if !end.Equal(tc.start.Add(5 * time.Minute)) {
				t.Fatalf("unexpected end: %s", end)
			}
		})
	}
}

func TestLateEventCorrectsEmittedWindow(t *testing.T) {
	base := baseTime()
	c := clock.NewManualClock(base)
	engine := window.NewEngine(c, 2*time.Minute)

	engine.Add(base.Add(time.Minute), "MX", 10000)

	c.Set(base.Add(5 * time.Minute))
	first := engine.Advance(base.Add(5 * time.Minute))

	if len(first) != 1 || first[0].Revision != 1 || first[0].Final {
		t.Fatalf("unexpected first emission: %+v", first)
	}

	c.Set(base.Add(6 * time.Minute))
	engine.Advance(base.Add(6 * time.Minute))
	correction := engine.Add(base.Add(2*time.Minute), "MX", 5000)

	if len(correction) != 1 {
		t.Fatalf("expected one correction, got %d", len(correction))
	}

	got := correction[0]

	if got.Count != 2 || got.GMVCents != 15000 || got.Revision != 2 {
		t.Fatalf("unexpected correction: %+v", got)
	}

	if !got.PublishedAt.Equal(c.Now()) {
		t.Fatal("publication did not use the injected clock")
	}
}

func TestFinalizedWindowRejectsLateEvent(t *testing.T) {
	base := baseTime()
	c := clock.NewManualClock(base)
	engine := window.NewEngine(c, 2*time.Minute)

	engine.Add(base.Add(time.Minute), "MX", 10000)

	c.Set(base.Add(5 * time.Minute))
	engine.Advance(base.Add(5 * time.Minute))

	// The finalization deadline is inclusive: end + lateness.
	c.Set(base.Add(7 * time.Minute))
	final := engine.Advance(base.Add(7 * time.Minute))

	if len(final) != 1 || !final[0].Final {
		t.Fatalf("expected a final result: %+v", final)
	}

	rejected := engine.Add(base.Add(2*time.Minute), "MX", 5000)

	if len(rejected) != 0 {
		t.Fatal("rejected event produced an update")
	}

	if engine.LateEvents != 1 || engine.LateCents != 5000 {
		t.Fatal("late event or amount was not counted")
	}

	if engine.ActiveCount() != 0 {
		t.Fatal("finalized window was retained or recreated")
	}
}

func TestFinishFinalizesAndReleasesState(t *testing.T) {
	base := baseTime()
	c := clock.NewManualClock(base.Add(time.Minute))
	engine := window.NewEngine(c, 2*time.Minute)

	engine.Add(base, "MX", 10000)
	engine.Add(base, "CL", 20000)

	updates := engine.Finish()
	finals := 0

	for _, result := range updates {
		if result.Final {
			finals++

			if !result.PublishedAt.Equal(c.Now()) {
				t.Fatal("finish changed the processing timestamp")
			}
		}
	}

	if finals != 2 || engine.ActiveCount() != 0 {
		t.Fatalf("finals=%d active=%d", finals, engine.ActiveCount())
	}

	if len(engine.Finish()) != 0 {
		t.Fatal("repeated finish produced additional updates")
	}
}

func TestBadClockDoesNotAdvanceWatermark(t *testing.T) {
	base := baseTime()
	c := clock.NewManualClock(base.Add(time.Second))
	w := window.NewWatermark(5 * time.Second)

	if !w.ObserveIfValid(base, c.Now(), 5*time.Minute) {
		t.Fatal("healthy event was rejected")
	}

	before := w.Value()

	// A device reports an event two hours ahead of its arrival.
	c.Set(base.Add(2 * time.Second))

	if w.ObserveIfValid(
		base.Add(2*time.Hour),
		c.Now(),
		5*time.Minute,
	) {
		t.Fatal("suspicious event was accepted")
	}

	if !w.Value().Equal(before) {
		t.Fatal("suspicious event advanced the watermark")
	}

	// An older healthy event must not move the watermark backwards.
	if !w.ObserveIfValid(
		base.Add(-time.Minute),
		c.Now(),
		5*time.Minute,
	) {
		t.Fatal("older healthy event was rejected")
	}

	if !w.Value().Equal(before) {
		t.Fatal("older event moved the watermark backwards")
	}

	// A newer healthy event must still advance it.
	c.Set(base.Add(11 * time.Second))

	if !w.ObserveIfValid(
		base.Add(10*time.Second),
		c.Now(),
		5*time.Minute,
	) {
		t.Fatal("newer healthy event was rejected")
	}

	expected := base.Add(5 * time.Second)

	if !w.Value().Equal(expected) {
		t.Fatalf("got %s, want %s", w.Value(), expected)
	}
}
