# E-Go05 — Pagos tardíos y ventanas de tiempo

Programa en Go que lee avisos de pago y calcula la cantidad de pagos
y el monto confirmado por país, en periodos de cinco minutos.

Los avisos pueden llegar después de que ocurrió el pago. El programa
permite corregir los resultados durante un margen configurable y
cuenta los pagos que quedan fuera por llegar demasiado tarde.

## Requisitos

- Go compatible con la versión indicada en go.mod.
- Python 3.12, utilizado para generar los datos.
- NumPy 2.4.4.
- Docker con Docker Compose.
- Puerto 5435 disponible para Postgres.

Los comandos se ejecutan desde la raíz del proyecto.

## Estructura

- cmd/processor: procesa eventos y guarda resultados en Postgres.
- cmd/truth: calcula la referencia con todos los pagos de reloj válido.
- cmd/check-clock: ejemplo del reloj controlado.
- internal/window: ventanas, watermark y manejo de pagos tardíos.
- internal/clock: reloj real y reloj controlado.
- internal/money: conversión de dólares a centavos.
- internal/storage: conexión y escritura en Postgres.
- scripts: generador de datos.
- migrations: creación de la tabla.
- tests: pruebas del comportamiento de las ventanas y del watermark.
- verdad: referencia y consulta de comparación.
- results: salidas de las ejecuciones.
- data: eventos generados y cuarentena.

## Ejecución completa

Con Go, Python 3.12 y Docker disponibles, ejecutar desde la raíz:

```bash
bash scripts/run_all.sh
```

El script instala NumPy en el entorno del proyecto, ejecuta las pruebas,
prepara Postgres, genera los datos, calcula la referencia, procesa los
tres márgenes y compara los resultados.

También repite la ejecución de dos minutos y verifica que sus filas
finales no cambien.

Las salidas se guardan en results/. El dataset y los archivos de
resultados se reemplazan en cada ejecución. Postgres permanece encendido.

## Ejecución paso a paso

Para ejecutar cada parte por separado, seguir los pasos siguientes.
Si ya se ejecutó `bash scripts/run_all.sh` correctamente, no es necesario repetirlos.

## 1. Preparar las dependencias

Crear las carpetas de salida:

```bash
mkdir -p data verdad results
```

Descargar las dependencias de Go:

```bash
go mod download
```

Crear el entorno de Python e instalar NumPy:

```bash
python3.12 -m venv .venv312
.venv312/bin/python -m pip install numpy==2.4.4
```

Se utiliza exactamente NumPy 2.4.4 porque el generador exige esa versión
para reproducir los datos.

## 2. Generar los eventos

El generador debe estar en scripts/generate_events_go05.py.

```bash
GO05_OUT=data/eventos_tardios.jsonl .venv312/bin/python scripts/generate_events_go05.py
```

Comprobar la cantidad de líneas:

```bash
wc -l data/eventos_tardios.jsonl
```

El archivo completo contiene 2,000,000 de eventos.

Cada línea es un JSON con estos campos:

- event_id: identificador del evento.
- event_type: tipo de evento.
- country: país.
- amount_usd: monto en dólares.
- event_time: cuándo ocurrió el pago.
- arrival_time: cuándo llegó su aviso.

Las fechas están en UTC. El archivo está ordenado por arrival_time,
pero los pagos pueden haber ocurrido en otro orden.

## 3. Preparar Postgres

Con Docker iniciado:

```bash
docker compose up -d
docker compose ps
```

Esperar a que el servicio aparezca como healthy.

Crear la tabla:

```bash
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U go05 -d hot_sale_go05 < migrations/001_windows.sql
```

La conexión local configurada para este ejercicio es:

- Host: localhost.
- Puerto: 5435.
- Base de datos: hot_sale_go05.
- Usuario: go05.
- Contraseña: go05_local.

## 4. Calcular la referencia

La referencia suma todos los pagos con reloj válido por periodo y país,
sin excluirlos por retraso.

```bash
go run ./cmd/truth -input data/eventos_tardios.jsonl -output verdad/windows.csv
```

Resultados obtenidos con el dataset completo:

| Dato | Resultado |
|---|---:|
| Eventos leídos | 2,000,000 |
| Eventos con reloj válido | 1,998,179 |
| Eventos con reloj sospechoso | 1,821 |
| Monto válido | USD 123,018,607.68 |
| Monto con reloj sospechoso | USD 115,306.98 |
| Grupos de periodo y país | 360 |

## 5. Ejecutar el procesador

El parámetro -lateness indica cuánto margen se permite para corregir
una ventana después de su final, según el avance del watermark.

El parámetro -run-id identifica la ejecución en Postgres.
Se utiliza un identificador distinto para cada configuración.

### Margen de 10 segundos

```bash
go run ./cmd/processor -input data/eventos_tardios.jsonl -lateness 10s -run-id go05-10s
```

### Margen de 2 minutos

```bash
go run ./cmd/processor -input data/eventos_tardios.jsonl -lateness 2m -run-id go05-2m
```

### Margen de 30 minutos

```bash
go run ./cmd/processor -input data/eventos_tardios.jsonl -lateness 30m -run-id go05-30m
```

Para guardar la salida de una ejecución:

```bash
go run ./cmd/processor -input data/eventos_tardios.jsonl -lateness 2m -run-id go05-2m > results/run-2m.txt 2>&1
```

El archivo de entrada se lee línea por línea. No se espera tiempo real:
arrival_time se usa como reloj simulado de procesamiento.

## 6. Cómo se decide si un pago entra

event_time determina el periodo al que pertenece el pago.

Por ejemplo, un pago ocurrido a las 10:03 pertenece al periodo
10:00–10:05, aunque su aviso llegue después.

El watermark se calcula así:

```text
Mayor event_time válido leído - 5 segundos
```

El resultado de una ventana se publica cuando el watermark alcanza
su final. Todavía puede corregirse hasta alcanzar:

```text
Final de la ventana + lateness
```

Al alcanzar ese límite, la ventana se finaliza y se elimina su estado
de memoria. Los pagos posteriores para ella se cuentan como demasiado
tardíos y se suma su monto descartado.

La lateness no cambia las fechas de los eventos y no es el tiempo
que tarda el programa en ejecutarse.

## 7. Relojes sospechosos

Un evento se separa cuando:

```text
event_time > arrival_time + 5 minutos
```

Estos eventos no modifican el watermark ni los totales válidos.

Se guardan con su motivo en:

```text
data/quarantine_bad_clock.jsonl
```

El archivo se reemplaza en cada ejecución del procesador.

Los pagos demasiado tardíos se contabilizan por cantidad y monto;
actualmente no se guardan individualmente en otro archivo.

## 8. Consultar los resultados guardados

```bash
docker compose exec postgres psql -U go05 -d hot_sale_go05 -c "
SELECT
    run_id,
    COUNT(*) AS grupos,
    COUNT(*) FILTER (WHERE is_final) AS finalizados,
    SUM(event_count) AS eventos,
    SUM(gmv_cents) / 100.0 AS gmv_usd
FROM window_results
GROUP BY run_id
ORDER BY run_id;
"
```

Resultados obtenidos:

| Ejecución | Grupos finales | Pagos aceptados | Monto aceptado en USD |
|---|---:|---:|---:|
| go05-10s | 360 | 1,940,678 | 119,499,175.48 |
| go05-2m | 360 | 1,961,808 | 120,794,264.96 |
| go05-30m | 360 | 1,992,873 | 122,697,197.32 |

## 9. Comparar contra la referencia

Copiar el CSV de referencia al contenedor:

```bash
docker compose cp verdad/windows.csv postgres:/tmp/go05_truth.csv
```

Ejecutar la comparación:

```bash
docker compose exec -T postgres psql -U go05 -d hot_sale_go05 -f - < verdad/compare.sql
```

La consulta compara cada periodo y país. Calcula:

- Cantidad y porcentaje de pagos descartados.
- Monto y porcentaje de dinero descartado.
- Error porcentual medio y máximo por grupo.
- Demora de finalización p50 y p95, en segundos simulados.
- Cantidad de grupos esperados, faltantes y finalizados.

El máximo de grupos conservados en memoria aparece en la salida
del procesador.

Las cifras completas y su explicación están en decisions.md.

## 10. Comprobar que repetir una ejecución no cambia el resultado final

Después de completar la ejecución de dos minutos, exportar sus filas:

```bash
docker compose exec -T postgres psql -U go05 -d hot_sale_go05 -c "\copy (SELECT * FROM window_results WHERE run_id = 'go05-2m' ORDER BY window_start, country) TO STDOUT WITH CSV HEADER" > data/before_replay.csv
```

Repetir el mismo archivo y configuración:

```bash
go run ./cmd/processor -input data/eventos_tardios.jsonl -lateness 2m -run-id go05-2m
```

Exportar nuevamente:

```bash
docker compose exec -T postgres psql -U go05 -d hot_sale_go05 -c "\copy (SELECT * FROM window_results WHERE run_id = 'go05-2m' ORDER BY window_start, country) TO STDOUT WITH CSV HEADER" > data/after_replay.csv
```

Comparar:

```bash
diff -u data/before_replay.csv data/after_replay.csv
```

Si no imprime diferencias, las filas finales son iguales.
Esta comprobación se realizó correctamente para go05-2m.

No ejecutar al mismo tiempo dos procesos con el mismo run-id.
Para otro archivo o configuración, utilizar un identificador diferente.

## 11. Ejecutar las pruebas

```bash
go test ./... -v
```

Se comprueban los límites de las ventanas, las correcciones permitidas,
el rechazo de pagos demasiado tardíos, la liberación del estado,
el cierre de fin de archivo y la protección del watermark frente
a relojes sospechosos.

Las pruebas utilizan un reloj controlado y no requieren Postgres.

## 12. Cierre del archivo

Cuando termina correctamente la lectura, el programa finaliza los grupos
que todavía estaban abiertos.

Como el watermark depende de las horas de ocurrencia, puede dejar de
avanzar mientras siguen llegando pagos antiguos. Por eso algunas ventanas
esperan hasta el fin del archivo para quedar cerradas.


## 13. Detener Postgres

```bash
docker compose down
```

El volumen conserva los datos para la siguiente ejecución.