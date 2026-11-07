CREATE TABLE IF NOT EXISTS window_results (
    run_id TEXT NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    country TEXT NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    event_count BIGINT NOT NULL CHECK (event_count >= 0),
    gmv_cents BIGINT NOT NULL CHECK (gmv_cents >= 0),
    revision BIGINT NOT NULL CHECK (revision >= 1),
    is_final BOOLEAN NOT NULL,
    published_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (run_id, window_start, country)
);