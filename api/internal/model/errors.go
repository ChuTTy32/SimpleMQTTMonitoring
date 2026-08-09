package model

import "errors"

// Sentinel-ошибки — общий язык между слоями. Repository переводит в них ошибки драйвера
// (pgx.ErrNoRows, коды PgError), handler переводит их в HTTP-статусы. Благодаря этому
// service ничего не знает ни про SQL, ни про HTTP.
var (
	// ErrNotFound — запрошенной строки нет.
	ErrNotFound = errors.New("not found")
	// ErrDuplicate — нарушение UNIQUE-ограничения (например, controllers.mqtt_gateway_id).
	ErrDuplicate = errors.New("already exists")
)
