package main

import (
	"fmt"
	"time"

	"e-go05/internal/clock"
)

func showTime(c clock.Clock) {
	fmt.Println("Hora del reloj:", c.Now().Format(time.RFC3339))
}

func main() {
	start := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	manual := clock.NewManualClock(start)

	showTime(manual)

	manual.Set(start.Add(5 * time.Minute))

	showTime(manual)
}
