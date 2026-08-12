-- Запросы модуля controllers. Источник схемы — ../../db/migrations (см. sqlc.yaml).

-- ORDER BY стабильный: без него LIMIT/OFFSET дают неопределённый порядок и строки
-- могут дублироваться/пропадать между страницами. created_at не уникален, поэтому
-- добиваем id как tie-breaker.
-- name: ListControllers :many
SELECT * FROM controllers
ORDER BY created_at DESC, id
LIMIT $1 OFFSET $2;

-- name: CountControllers :one
SELECT count(*) FROM controllers;

-- name: GetController :one
SELECT * FROM controllers
WHERE id = $1;

-- name: CreateController :one
INSERT INTO controllers (name, mqtt_gateway_id, ip_address, location, is_active)
-- Явный ::boolean обязателен: без каста sqlc не выводит тип из COALESCE и генерирует
-- interface{} вместо *bool.
VALUES ($1, $2, $3, $4, COALESCE(sqlc.narg('is_active')::boolean, true))
RETURNING *;

-- PATCH-семантика: NULL-параметр означает «поле не передано, не трогать».
-- Ограничение: обнулить nullable-поле (ip_address/location) этим запросом нельзя —
-- явный null неотличим от «не передано». Для MVP приемлемо; если понадобится —
-- решать отдельными флагами или отдельным запросом, не усложняя этот.
-- name: UpdateController :one
UPDATE controllers
SET
    name            = COALESCE(sqlc.narg('name'), name),
    mqtt_gateway_id = COALESCE(sqlc.narg('mqtt_gateway_id'), mqtt_gateway_id),
    ip_address      = COALESCE(sqlc.narg('ip_address'), ip_address),
    location        = COALESCE(sqlc.narg('location'), location),
    is_active       = COALESCE(sqlc.narg('is_active'), is_active),
    updated_at      = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- :execrows — нужно количество затронутых строк, чтобы отличить успешное удаление
-- от удаления несуществующего id (404).
-- name: DeleteController :execrows
DELETE FROM controllers
WHERE id = $1;
