package model

import (
	"time"

	"github.com/google/uuid"
)

// Controller — контроллер/шлюз (ПЛК, ESP32), объединяющий датчики. Поля повторяют схему
// db/migrations/000001_init_schema.up.sql.
type Controller struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	MQTTGatewayID string    `json:"mqtt_gateway_id"`
	IPAddress     *string   `json:"ip_address"`
	Location      *string   `json:"location"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CreateControllerInput — тело POST /controllers.
type CreateControllerInput struct {
	Name          string  `json:"name" validate:"required,max=100"`
	MQTTGatewayID string  `json:"mqtt_gateway_id" validate:"required,max=100"`
	IPAddress     *string `json:"ip_address" validate:"omitempty,ip"`
	Location      *string `json:"location" validate:"omitempty,max=255"`
	// nil — не передано, БД подставит DEFAULT true.
	IsActive *bool `json:"is_active"`
}

// UpdateControllerInput — тело PATCH /controllers/{id}. Все поля указатели: nil означает
// «не передано, не трогать», иначе нельзя отличить пропуск поля от осознанного
// обнуления (пустая строка, false).
type UpdateControllerInput struct {
	Name          *string `json:"name" validate:"omitempty,max=100"`
	MQTTGatewayID *string `json:"mqtt_gateway_id" validate:"omitempty,max=100"`
	IPAddress     *string `json:"ip_address" validate:"omitempty,ip"`
	Location      *string `json:"location" validate:"omitempty,max=255"`
	IsActive      *bool   `json:"is_active"`
}
