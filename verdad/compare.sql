\set ON_ERROR_STOP on

CREATE TEMP TABLE truth_windows (
    window_start TIMESTAMPTZ NOT NULL,
    country TEXT NOT NULL,
    event_count BIGINT NOT NULL,
    gmv_cents BIGINT NOT NULL,
    PRIMARY KEY (window_start, country)
);

\copy truth_windows FROM '/tmp/go05_truth.csv' WITH (FORMAT csv, HEADER true)

WITH runs (run_id, lateness_seconds) AS (
    VALUES
        ('go05-10s', 10),
        ('go05-2m', 120),
        ('go05-30m', 1800)
),
comparison AS (
    SELECT
        r.run_id,
        r.lateness_seconds,
        t.window_start,
        t.country,
        t.event_count AS truth_count,
        t.gmv_cents AS truth_cents,
        w.event_count AS actual_count,
        w.gmv_cents AS actual_cents,
        w.is_final,
        w.published_at,
        w.window_end,
        w.run_id IS NULL AS missing
    FROM runs r
    CROSS JOIN truth_windows t
    LEFT JOIN window_results w
        ON w.run_id = r.run_id
        AND w.window_start = t.window_start
        AND w.country = t.country
)
SELECT
    run_id,
    COUNT(*) AS expected_groups,
    COUNT(*) FILTER (WHERE missing) AS missing_groups,
    COUNT(*) FILTER (WHERE is_final) AS finalized_groups,

    SUM(truth_count - COALESCE(actual_count, 0))
        AS discarded_events,

    ROUND(
        100.0 * SUM(truth_count - COALESCE(actual_count, 0))
        / NULLIF(SUM(truth_count), 0),
        4
    ) AS discarded_events_pct,

    ROUND(
        SUM(truth_cents - COALESCE(actual_cents, 0)) / 100.0,
        2
    ) AS discarded_gmv_usd,

    ROUND(
        100.0 * SUM(truth_cents - COALESCE(actual_cents, 0))
        / NULLIF(SUM(truth_cents), 0),
        4
    ) AS discarded_gmv_pct,

    ROUND(
        AVG(
            100.0 * ABS(COALESCE(actual_cents, 0) - truth_cents)
            / NULLIF(truth_cents, 0)
        ),
        4
    ) AS mean_window_error_pct,

    ROUND(
        MAX(
            100.0 * ABS(COALESCE(actual_cents, 0) - truth_cents)
            / NULLIF(truth_cents, 0)
        ),
        4
    ) AS max_window_error_pct,

    ROUND(
        (
            percentile_cont(0.50) WITHIN GROUP (
                ORDER BY EXTRACT(
                    EPOCH FROM published_at - window_end
                )::double precision
            ) FILTER (WHERE is_final)
        )::numeric,
        3
    ) AS finalization_p50_seconds,

    ROUND(
        (
            percentile_cont(0.95) WITHIN GROUP (
                ORDER BY EXTRACT(
                    EPOCH FROM published_at - window_end
                )::double precision
            ) FILTER (WHERE is_final)
        )::numeric,
        3
    ) AS finalization_p95_seconds

FROM comparison
GROUP BY run_id, lateness_seconds
ORDER BY lateness_seconds;