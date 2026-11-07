package storage

import (
	"context"
	"fmt"
	"time"

	"e-go05/internal/window"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func Open(dsn string) (*Store, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("no se pudo conectar a Postgres: %w", err)
	}

	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Save(runID string, results []window.Result) error {
	const query = `
		INSERT INTO window_results (
			run_id, window_start, country, window_end,
			event_count, gmv_cents, revision, is_final, published_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (run_id, window_start, country)
		DO UPDATE SET
			window_end = EXCLUDED.window_end,
			event_count = EXCLUDED.event_count,
			gmv_cents = EXCLUDED.gmv_cents,
			revision = EXCLUDED.revision,
			is_final = EXCLUDED.is_final,
			published_at = EXCLUDED.published_at
	`

	for _, result := range results {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)

		_, err := s.pool.Exec(
			ctx,
			query,
			runID,
			result.Start,
			result.Country,
			result.End,
			result.Count,
			result.GMVCents,
			result.Revision,
			result.Final,
			result.PublishedAt,
		)
		cancel()

		if err != nil {
			return fmt.Errorf(
				"guardando ventana %s, país %s: %w",
				result.Start,
				result.Country,
				err,
			)
		}
	}

	return nil
}
