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
	// ErrReferenceNotFound — нарушение FOREIGN KEY: запись ссылается на несуществующую
	// строку (например, sensors.controller_id на отсутствующий контроллер). Отличается от
	// ErrNotFound: не найдено не то, что запрашивали, а то, на что ссылались в теле запроса.
	ErrReferenceNotFound = errors.New("referenced resource does not exist")
	// ErrConstraintViolation — нарушение CHECK-ограничения (например,
	// sensors.min_threshold < max_threshold). Данные синтаксически корректны, но
	// противоречат правилу, которое стережёт БД.
	ErrConstraintViolation = errors.New("constraint violation")
)
