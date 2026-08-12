-- Запросы модуля sensors. Источник схемы — ../../db/migrations (см. sqlc.yaml).

-- Фильтр по контроллеру необязательный: NULL-параметр означает «все контроллеры».
-- Тот же приём, что и для PATCH — одна ветка SQL вместо двух почти одинаковых запросов.
-- ORDER BY стабильный (name не уникален, добиваем id) — иначе LIMIT/OFFSET дают
-- неопределённый порядок и строки скачут между страницами.
-- name: ListSensors :many
SELECT * FROM sensors
WHERE (sqlc.narg('controller_id')::uuid IS NULL OR controller_id = sqlc.narg('controller_id')::uuid)
ORDER BY name, id
LIMIT $1 OFFSET $2;

-- Фильтр обязан повторять условие ListSensors: иначе total в ответе посчитается по всем
-- датчикам и разойдётся с отфильтрованной страницей.
-- name: CountSensors :one
SELECT count(*) FROM sensors
WHERE (sqlc.narg('controller_id')::uuid IS NULL OR controller_id = sqlc.narg('controller_id')::uuid);

-- name: GetSensor :one
SELECT * FROM sensors
WHERE id = $1;

-- Явный ::boolean — без каста sqlc не выводит тип из COALESCE и генерирует interface{}.
-- name: CreateSensor :one
INSERT INTO sensors (controller_id, name, topic, metric_type, unit, min_threshold, max_threshold, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE(sqlc.narg('is_active')::boolean, true))
RETURNING *;

-- PATCH-семантика: NULL-параметр означает «поле не передано, не трогать».
-- Ограничение то же, что у controllers: обнулить nullable-поле (пороги) этим запросом
-- нельзя — явный null неотличим от «не передано».
-- name: UpdateSensor :one
UPDATE sensors
SET
    controller_id = COALESCE(sqlc.narg('controller_id'), controller_id),
    name          = COALESCE(sqlc.narg('name'), name),
    topic         = COALESCE(sqlc.narg('topic'), topic),
    metric_type   = COALESCE(sqlc.narg('metric_type'), metric_type),
    unit          = COALESCE(sqlc.narg('unit'), unit),
    min_threshold = COALESCE(sqlc.narg('min_threshold'), min_threshold),
    max_threshold = COALESCE(sqlc.narg('max_threshold'), max_threshold),
    is_active     = COALESCE(sqlc.narg('is_active'), is_active),
    updated_at    = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- :execrows — нужно количество затронутых строк, чтобы отличить успешное удаление
-- от удаления несуществующего id (404).
-- name: DeleteSensor :execrows
DELETE FROM sensors
WHERE id = $1;
