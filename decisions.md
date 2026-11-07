# Decisiones de E-Go05

## 1. Cómo se agrupan los pagos y cuándo se cierran los resultados

Cada evento del archivo ya contiene dos fechas:

- event_time: cuándo ocurrió el pago.
- arrival_time: cuándo llegó al sistema.

Para elegir el grupo de un pago utilizo event_time. Los grupos duran
cinco minutos y se separan por país.

Por ejemplo, un pago de México que ocurrió a las 10:03 pertenece al
grupo de México de 10:00 a 10:05, aunque haya llegado a las 10:06.

El inicio está incluido y el final pertenece al siguiente grupo.
Un pago ocurrido exactamente a las 10:05 entra en el grupo de
10:05 a 10:10.

### Cómo decido cuándo publicar o finalizar un grupo

Uso una marca de tiempo llamada watermark. La calculo tomando la mayor
hora de ocurrencia válida que el programa ha leído y restándole
cinco segundos.

watermark = mayor event_time válido leído - 5 segundos

Elegí cinco segundos como margen inicial para el desorden de llegada.
Esto no garantiza que ya hayan llegado todos los pagos anteriores.

El watermark nunca retrocede. Un pago antiguo puede corregir un grupo,
pero no hace retroceder esa marca.

Cuando el watermark alcanza el final de un grupo, publico su primer
resultado. Todavía permito corregirlo durante el margen elegido.

Cuando el watermark alcanza el final del grupo más ese margen,
el resultado se vuelve final y elimino su estado de memoria.

Por ejemplo, con un margen de dos minutos:

- El grupo comprende de 10:00 a 10:05.
- Se publica cuando el watermark alcanza las 10:05.
- Se puede corregir mientras el watermark sea anterior a las 10:07.
- Cuando alcanza las 10:07, el grupo se finaliza.
- Los pagos posteriores para ese grupo quedan fuera del resultado.

Los pagos rechazados se cuentan y se suma su monto para saber cuánto
dinero quedó fuera por retraso.

## 2. Resultados de los tres márgenes

Probé 10 segundos, 2 minutos y 30 minutos.

Estos valores no modifican las fechas de los eventos. Cambian hasta
cuándo permito corregir cada grupo.

Para comparar los resultados calculé una referencia con todos los pagos
de reloj válido, incluyendo los que llegaron muy tarde.

La referencia contiene:

- 2,000,000 de eventos leídos.
- 1,998,179 eventos con reloj válido.
- USD 123,018,607.68 de monto válido.
- 360 grupos: 72 periodos de cinco minutos por cinco países.

El cálculo de referencia se realiza con cmd/truth. No utiliza el motor
de ventanas del procesador. Ambos programas utilizan la misma conversión
de dinero a centavos y la misma regla para separar relojes sospechosos.

### Pagos y dinero que quedaron fuera

Los porcentajes se calculan sobre los eventos y el dinero con reloj válido.
Los eventos con reloj sospechoso se cuentan por separado.

| Margen | Pagos descartados | Porcentaje de pagos | Dinero descartado en USD | Porcentaje del dinero |
|---|---:|---:|---:|---:|
| 10 segundos | 57,501 | 2.8777% | 3,519,432.20 | 2.8609% |
| 2 minutos | 36,371 | 1.8202% | 2,224,342.72 | 1.8081% |
| 30 minutos | 5,306 | 0.2655% | 321,410.36 | 0.2613% |

### Diferencias por grupo, espera y estado en memoria

| Margen | Error medio por grupo | Error máximo por grupo | Espera p50 en segundos | Espera p95 en segundos | Máximo de grupos en memoria |
|---|---:|---:|---:|---:|---:|
| 10 segundos | 2.8483% | 3.9509% | 15.163 | 15.342 | 10 |
| 2 minutos | 1.8062% | 2.7061% | 125.191 | 125.392 | 10 |
| 30 minutos | 0.2608% | 0.6656% | 1805.172 | 11496.456 | 40 |

Las tres ejecuciones terminaron con 360 grupos finalizados y ninguno
faltante respecto a la referencia.

Para calcular el error comparé el dinero de cada grupo contra su
referencia:

error porcentual = 100 × diferencia absoluta / monto de referencia

El error medio es el promedio de los errores de los 360 grupos.
El error máximo es la mayor diferencia porcentual encontrada.

La espera se mide desde el final del grupo hasta la publicación de su
versión final. Se utiliza la hora simulada de llegada, no los segundos
que tarda el programa en ejecutarse en la computadora.

El p50 indica la espera que no superó aproximadamente la mitad de los
grupos. El p95 indica la espera que no superó aproximadamente el 95%.

El máximo en memoria cuenta combinaciones de periodo y país que el
programa conservó al mismo tiempo. No es una medición en megabytes.

## 3. Qué configuración recomendaría

Para un tablero que necesita responder rápido, propondría empezar con
dos minutos, siempre que operaciones acepte la diferencia encontrada.

Con ese margen, la espera p95 fue de unos 125 segundos y quedó fuera
el 1.8081% del dinero válido. El mayor error de un grupo fue de 2.7061%.

Esta propuesta no sería adecuada si el negocio necesita cifras casi
exactas o no acepta ese porcentaje de dinero fuera del reporte.

El margen de treinta minutos recuperó más pagos y dejó fuera solo el
0.2613% del dinero válido. Sin embargo, necesitó conservar más grupos
y algunas ventanas tardaron mucho más en finalizar.

Antes de elegirlo confirmaría con operaciones qué importa más:
tener el resultado final pronto o incluir una mayor cantidad de pagos.

El tablero debe indicar si un resultado sigue abierto a correcciones
o ya es final. Mientras sea provisional puede aumentar.


## 4. Qué hago con los relojes sospechosos

Un evento se considera sospechoso cuando su hora de ocurrencia está
más de cinco minutos por delante de su hora de llegada:

event_time > arrival_time + 5 minutos

Esta validación se hace antes de permitir que el evento cambie el
watermark. Así, un dispositivo con el reloj adelantado no provoca
que se cierren grupos antes de tiempo.

Elegí cinco minutos porque permite cierto adelanto del dispositivo
y está dentro del rango que las instrucciones indican que separa
los relojes sanos de los rotos en este dataset.

Se encontraron:

- 1,821 eventos sospechosos.
- USD 115,306.98 asociados a esos eventos.

Los guardo en data/quarantine_bad_clock.jsonl junto con el motivo.

No los elimino sin dejar registro porque representan pagos que deben
poder revisarse. La cuarentena permite investigar el problema sin
mezclarlos con los resultados válidos.

## 5. Qué cambiaría si los retrasos largos fueran del 30%

Volvería a probar los márgenes antes de elegir una configuración, porque
estaríamos dejando fuera muchos más pagos.

Revisaría cuánto tardan en llegar y cuánto dinero representan. Probaría
un margen mayor, midiendo cuánto aumenta la espera y cuántos grupos
necesita conservar el programa.

También mantendría un reporte provisional para operaciones y un cálculo
posterior para incorporar los pagos que llegaron después del cierre.

## 6. Qué pasa cuando termina el archivo

El archivo se procesa en orden de llegada, sin esperar tiempo real.
arrival_time se utiliza como reloj de procesamiento.

Como el watermark depende de la hora de ocurrencia más reciente,
puede dejar de avanzar aunque sigan llegando pagos antiguos.

Cuando la lectura termina correctamente, Finish finaliza los grupos
que todavía siguen abiertos. Para ello avanza la marca interna hasta
sus límites de cierre y conserva como hora de publicación la llegada
del último evento leído.

Esto tiene una consecuencia: los últimos grupos pueden seguir
aceptando correcciones hasta terminar el archivo.

En la prueba de treinta minutos, la espera p95 fue de 11,496.456 segundos,
aproximadamente tres horas y once minutos. El margen de treinta minutos
no garantiza que todos los grupos finalicen en treinta minutos de
tiempo de procesamiento.

Para un sistema que recibe eventos continuamente, habría que definir
también cómo avanzar durante los periodos sin eventos recientes.
Esa política adicional no está implementada.

## 7. Cómo guardo los resultados y evito duplicarlos

Los resultados se guardan en Postgres. Cada fila se identifica con:

(run_id, window_start, country)

run_id identifica la ejecución. Los otros campos identifican el periodo
y el país.

Cada publicación, corrección y finalización guarda la versión calculada.
Si la fila ya existe, se reemplazan sus totales y su número de revisión.
No se vuelve a sumar el dinero que ya estaba guardado.

Comprobé la ejecución de dos minutos procesando dos veces el mismo
archivo con el mismo run_id.

Exporté todas las filas antes y después de repetirla. Al compararlas
con diff no hubo diferencias, incluidas las revisiones y las fechas.

La comprobación cubre una repetición completa y secuencial.
No implementé recuperación por puntos de avance ni ejecuciones
simultáneas con el mismo run_id.

## 8. Pruebas realizadas

Las cinco pruebas principales pasaron con:

go test ./... -v

Cubren:

- Los límites de los grupos de cinco minutos.
- Un pago tardío que todavía puede corregir un resultado.
- Un pago rechazado al alcanzar el límite de finalización.
- El cierre de los grupos y la liberación de su estado.
- Un reloj sospechoso que no cambia el watermark.

También se comprueba que un evento antiguo no haga retroceder el
watermark y que repetir Finish no produzca nuevas publicaciones.

Las pruebas usan un reloj controlado para cambiar la hora sin esperar
minutos reales.