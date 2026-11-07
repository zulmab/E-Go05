package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"e-go05/internal/clock"
	"e-go05/internal/money"
	"e-go05/internal/storage"
	"e-go05/internal/window"
)

type Event struct {
	EventID     string      `json:"event_id"`
	EventType   string      `json:"event_type"`
	Country     string      `json:"country"`
	AmountUSD   json.Number `json:"amount_usd"`
	EventTime   time.Time   `json:"event_time"`
	ArrivalTime time.Time   `json:"arrival_time"`
}

func main() {
	input := flag.String(
		"input",
		"data/eventos_tardios.jsonl",
		"Ruta del archivo de eventos",
	)

	allowedLateness := flag.Duration(
		"lateness",
		2*time.Minute,
		"Margen para corregir ventanas después de su fin según el watermark",
	)

	dsn := flag.String(
		"dsn",
		"postgres://go05:go05_local@localhost:5435/hot_sale_go05?sslmode=disable",
		"Conexión a Postgres local",
	)

	runID := flag.String(
		"run-id",
		"",
		"Identificador de la ejecución",
	)

	flag.Parse()

	if *runID == "" {
		log.Fatal("Debes indicar -run-id")
	}

	store, err := storage.Open(*dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	fmt.Println("Conectado a Postgres")

	if *allowedLateness < 0 {
		log.Fatal("La lateness no puede ser negativa")
	}

	fmt.Printf("Lateness permitida: %s\n", *allowedLateness)

	quarantine, err := os.Create("data/quarantine_bad_clock.jsonl")
	if err != nil {
		log.Fatalf("No se pudo crear la cuarentena: %v", err)
	}
	defer quarantine.Close()

	file, err := os.Open(*input)
	if err != nil {
		log.Fatalf("No se pudo abrir el archivo: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	total := 0

	const futureTolerance = 5 * time.Minute

	validEvents := 0
	badClockEvents := 0
	var badClockCents int64
	processingClock := clock.NewManualClock(time.Time{})
	watermark := window.NewWatermark(5 * time.Second)
	engine := window.NewEngine(processingClock, *allowedLateness)

	for scanner.Scan() {
		var event Event

		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			log.Fatalf("Error en la línea %d: %v", total+1, err)
		}

		processingClock.Set(event.ArrivalTime)

		total++

		amountCents, err := money.ParseCents(event.AmountUSD.String())
		if err != nil {
			log.Fatalf("Monto inválido en la línea %d: %v", total, err)
		}

		if !watermark.ObserveIfValid(
			event.EventTime,
			event.ArrivalTime,
			futureTolerance,
		) {
			badClockEvents++
			badClockCents += amountCents

			record := struct {
				Event  Event  `json:"event"`
				Reason string `json:"reason"`
			}{
				Event:  event,
				Reason: "event_time exceeds arrival_time by more than 5 minutes",
			}

			if err := json.NewEncoder(quarantine).Encode(record); err != nil {
				log.Fatalf("Error guardando cuarentena en línea %d: %v", total, err)
			}

			continue
		}

		validEvents++

		if err := store.Save(*runID, engine.Advance(watermark.Value())); err != nil {
			log.Fatal(err)
		}

		if err := store.Save(
			*runID,
			engine.Add(event.EventTime, event.Country, amountCents),
		); err != nil {
			log.Fatal(err)
		}

		if total <= 3 {
			fmt.Printf(
				"Evento: %s | País: %s | USD: %s\n",
				event.EventID,
				event.Country,
				event.AmountUSD,
			)

			fmt.Printf(
				"  Ocurrió: %s | Llegó: %s | Diferencia: %s\n",
				event.EventTime.Format(time.RFC3339Nano),
				event.ArrivalTime.Format(time.RFC3339Nano),
				event.ArrivalTime.Sub(event.EventTime),
			)

			start, end := window.Bounds(event.EventTime)

			fmt.Printf(
				"  Ventana: [%s, %s)\n",
				start.Format(time.RFC3339),
				end.Format(time.RFC3339),
			)

			fmt.Printf("  Monto en centavos: %d\n", amountCents)

			fmt.Printf(
				"  Reloj de procesamiento: %s\n",
				processingClock.Now().Format(time.RFC3339Nano),
			)

			fmt.Printf(
				"  Watermark: %s\n",
				watermark.Value().Format(time.RFC3339Nano),
			)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error leyendo el archivo: %v", err)
	}

	if err := store.Save(*runID, engine.Finish()); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nEventos leídos: %d\n", total)
	fmt.Printf("Eventos con reloj válido: %d\n", validEvents)
	fmt.Printf("Eventos en cuarentena por reloj: %d\n", badClockEvents)
	fmt.Printf(
		"GMV en cuarentena: USD %d.%02d\n",
		badClockCents/100,
		badClockCents%100,
	)

	fmt.Printf(
		"Watermark final: %s\n",
		watermark.Value().Format(time.RFC3339Nano),
	)

	fmt.Printf("\n--- Motor de ventanas ---\n")
	fmt.Printf("Eventos aceptados: %d\n", engine.AcceptedEvents)
	fmt.Printf("Eventos demasiado tardíos: %d\n", engine.LateEvents)
	fmt.Printf(
		"GMV demasiado tardío: USD %d.%02d\n",
		engine.LateCents/100,
		engine.LateCents%100,
	)
	fmt.Printf("Primeras emisiones: %d\n", engine.Emissions)
	fmt.Printf("Correcciones: %d\n", engine.Corrections)
	fmt.Printf("Grupos finalizados: %d\n", engine.Finalized)
	fmt.Printf("Grupos todavía activos: %d\n", engine.ActiveCount())
	fmt.Printf("Máximo de grupos activos: %d\n", engine.MaxActive)
}
