#!/usr/bin/env bash
set -euo pipefail

# Run from the project root.
cd "$(dirname "${BASH_SOURCE[0]}")/.."

command -v go >/dev/null || {
  echo "Falta instalar Go."
  exit 1
}

command -v python3.12 >/dev/null || {
  echo "Falta instalar Python 3.12."
  exit 1
}

command -v docker >/dev/null || {
  echo "Falta instalar Docker."
  exit 1
}

docker info >/dev/null
docker compose version >/dev/null

mkdir -p data verdad results

echo "Preparando Python y NumPy..."
if [ ! -x .venv312/bin/python ]; then
  python3.12 -m venv .venv312
fi

.venv312/bin/python -m pip install numpy==2.4.4

echo "Descargando dependencias y ejecutando pruebas..."
go mod download
go test ./... -v | tee results/tests.txt

echo "Preparando Postgres..."
docker compose up -d --wait

docker compose exec -T postgres \
  psql -v ON_ERROR_STOP=1 -U go05 -d hot_sale_go05 \
  < migrations/001_windows.sql

echo "Generando los eventos..."
GO05_OUT=data/eventos_tardios.jsonl \
  .venv312/bin/python scripts/generate_events_go05.py \
  | tee results/generator.txt

echo "Calculando la referencia..."
go run ./cmd/truth \
  -input data/eventos_tardios.jsonl \
  -output verdad/windows.csv \
  | tee results/truth.txt

for lateness in 10s 2m 30m; do
  echo "Procesando con margen de $lateness..."

  go run ./cmd/processor \
    -input data/eventos_tardios.jsonl \
    -lateness "$lateness" \
    -run-id "go05-$lateness" \
    | tee "results/run-$lateness.txt"
done

echo "Comparando contra la referencia..."
docker compose cp \
  verdad/windows.csv \
  postgres:/tmp/go05_truth.csv

docker compose exec -T postgres \
  psql -v ON_ERROR_STOP=1 -U go05 -d hot_sale_go05 -f - \
  < verdad/compare.sql \
  | tee results/comparison.txt

echo "Verificando que repetir la ejecución no cambie sus resultados..."

docker compose exec -T postgres \
  psql -v ON_ERROR_STOP=1 -U go05 -d hot_sale_go05 \
  -c "\copy (SELECT * FROM window_results WHERE run_id = 'go05-2m' ORDER BY window_start, country) TO STDOUT WITH CSV HEADER" \
  > data/before_replay.csv

go run ./cmd/processor \
  -input data/eventos_tardios.jsonl \
  -lateness 2m \
  -run-id go05-2m \
  > results/replay-2m.txt 2>&1

docker compose exec -T postgres \
  psql -v ON_ERROR_STOP=1 -U go05 -d hot_sale_go05 \
  -c "\copy (SELECT * FROM window_results WHERE run_id = 'go05-2m' ORDER BY window_start, country) TO STDOUT WITH CSV HEADER" \
  > data/after_replay.csv

diff -u data/before_replay.csv data/after_replay.csv

echo "Proceso completo. La repetición dejó los mismos resultados."
echo "Las salidas están en results/. Postgres permanece encendido."