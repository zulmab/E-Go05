package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"time"

	"e-go05/internal/money"
)

type Event struct {
	Country     string      `json:"country"`
	AmountUSD   json.Number `json:"amount_usd"`
	EventTime   time.Time   `json:"event_time"`
	ArrivalTime time.Time   `json:"arrival_time"`
}

type Key struct {
	WindowStart int64
	Country     string
}

type Aggregate struct {
	Count    int64
	GMVCents int64
}

func main() {
	input := flag.String("input", "data/eventos_tardios.jsonl", "Archivo de eventos")
	output := flag.String("output", "verdad/windows.csv", "Archivo de resultados")
	flag.Parse()

	if err := run(*input, *output); err != nil {
		log.Fatal(err)
	}
}

func run(input, output string) error {
	file, err := os.Open(input)
	if err != nil {
		return err
	}
	defer file.Close()

	groups := make(map[Key]Aggregate)
	scanner := bufio.NewScanner(file)

	var total, valid, badClock int64
	var validCents, badClockCents int64

	for scanner.Scan() {
		total++

		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return fmt.Errorf("línea %d: %w", total, err)
		}

		cents, err := money.ParseCents(event.AmountUSD.String())
		if err != nil {
			return fmt.Errorf("monto en línea %d: %w", total, err)
		}

		if event.EventTime.After(event.ArrivalTime.Add(5 * time.Minute)) {
			badClock++
			badClockCents += cents
			continue
		}

		valid++
		validCents += cents

		// Calculate five-minute buckets independently of internal/window.
		const windowSeconds int64 = 300
		seconds := event.EventTime.Unix()
		start := seconds - seconds%windowSeconds

		key := Key{
			WindowStart: start,
			Country:     event.Country,
		}

		aggregate := groups[key]
		aggregate.Count++
		aggregate.GMVCents += cents
		groups[key] = aggregate
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	keys := make([]Key, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}

	sort.Slice(keys, func(i, j int) bool {
		if keys[i].WindowStart == keys[j].WindowStart {
			return keys[i].Country < keys[j].Country
		}
		return keys[i].WindowStart < keys[j].WindowStart
	})

	if err := writeCSV(output, keys, groups); err != nil {
		return err
	}

	fmt.Printf("Eventos leídos: %d\n", total)
	fmt.Printf("Eventos válidos: %d\n", valid)
	fmt.Printf("Relojes sospechosos: %d\n", badClock)
	fmt.Printf("GMV válido: USD %d.%02d\n", validCents/100, validCents%100)
	fmt.Printf("GMV en cuarentena: USD %d.%02d\n", badClockCents/100, badClockCents%100)
	fmt.Printf("Grupos ventana-país: %d\n", len(groups))
	fmt.Printf("Referencia guardada en: %s\n", output)

	return nil
}

func writeCSV(path string, keys []Key, groups map[Key]Aggregate) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}

	writer := csv.NewWriter(file)

	if err := writer.Write([]string{
		"window_start", "country", "event_count", "gmv_cents",
	}); err != nil {
		file.Close()
		return err
	}

	for _, key := range keys {
		aggregate := groups[key]

		record := []string{
			time.Unix(key.WindowStart, 0).UTC().Format(time.RFC3339),
			key.Country,
			strconv.FormatInt(aggregate.Count, 10),
			strconv.FormatInt(aggregate.GMVCents, 10),
		}

		if err := writer.Write(record); err != nil {
			file.Close()
			return err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		file.Close()
		return err
	}

	return file.Close()
}
