package model

import (
	"time"

	"github.com/google/uuid"
)

// Sensor — датчик, привязанный к контроллеру. Поля повторяют схему
// db/migrations/000001_init_schema.up.sql.
type Sensor struct {
	ID           uuid.UUID `json:"id"`
	ControllerID uuid.UUID `json:"controller_id"`
	Name         string    `json:"name"`
	Topic        string    `json:"topic"`
	MetricType   string    `json:"metric_type"`
	Unit         string    `json:"unit"`
	MinThreshold *float64  `json:"min_threshold"`
	MaxThreshold *float64  `json:"max_threshold"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SensorFilter — необязательные условия выборки списка. Структура, а не голый параметр:
// фильтров со временем станет больше, и сигнатуры слоёв не должны меняться от каждого.
type SensorFilter struct {
	// ControllerID: nil — выбирать датчики всех контроллеров.
	ControllerID *uuid.UUID
}

// CreateSensorInput — тело POST /sensors.
//
// metric_type сознательно не ограничен списком значений: в схеме это VARCHAR(50) без
// CHECK, и API не должен быть строже схемы — новые типы метрик (vibration, current, flow)
// не должны требовать правки Go-кода и деплоя.
type CreateSensorInput struct {
	ControllerID uuid.UUID `json:"controller_id" validate:"required"`
	Name         string    `json:"name" validate:"required,max=100"`
	Topic        string    `json:"topic" validate:"required,max=255"`
	MetricType   string    `json:"metric_type" validate:"required,max=50"`
	Unit         string    `json:"unit" validate:"required,max=20"`
	MinThreshold *float64  `json:"min_threshold"`
	MaxThreshold *float64  `json:"max_threshold"`
	// nil — не передано, БД подставит DEFAULT true.
	IsActive *bool `json:"is_active"`
}

// UpdateSensorInput — тело PATCH /sensors/{id}. Все поля указатели: nil означает
// «не передано, не трогать».
type UpdateSensorInput struct {
	ControllerID *uuid.UUID `json:"controller_id"`
	Name         *string    `json:"name" validate:"omitempty,max=100"`
	Topic        *string    `json:"topic" validate:"omitempty,max=255"`
	MetricType   *string    `json:"metric_type" validate:"omitempty,max=50"`
	Unit         *string    `json:"unit" validate:"omitempty,max=20"`
	MinThreshold *float64   `json:"min_threshold"`
	MaxThreshold *float64   `json:"max_threshold"`
	IsActive     *bool      `json:"is_active"`
}
