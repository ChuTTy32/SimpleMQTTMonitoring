package handler

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/ChuTTy32/SimpleMQTTMonitoring/api/internal/model"
)

// newValidator собирает общий для всех модулей валидатор входных DTO.
func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())

	// Имена полей в ошибках берём из json-тегов (mqtt_gateway_id), а не из имён Go-полей
	// (MQTTGatewayID) — иначе фронт не сопоставит ошибку с полем формы.
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "" || name == "-" {
			return fld.Name
		}
		return name
	})

	v.RegisterStructValidation(validateThresholds, model.CreateSensorInput{}, model.UpdateSensorInput{})

	return v
}

// validateThresholds проверяет min_threshold < max_threshold — то же правило, что стоит
// CHECK-ограничением в схеме.
//
// Зачем дублировать проверку БД: здесь она даёт точную привязку к полю формы
// (fields.min_threshold), тогда как из ошибки CHECK известно только то, что какое-то
// ограничение нарушено. При этом БД остаётся окончательной гарантией и покрывает случай,
// который тут проверить нельзя: PATCH с одним порогом, когда второй лежит в базе и Go
// его не видит.
func validateThresholds(sl validator.StructLevel) {
	var minThreshold, maxThreshold *float64

	switch in := sl.Current().Interface().(type) {
	case model.CreateSensorInput:
		minThreshold, maxThreshold = in.MinThreshold, in.MaxThreshold
	case model.UpdateSensorInput:
		minThreshold, maxThreshold = in.MinThreshold, in.MaxThreshold
	default:
		return
	}

	// Если передан только один порог — сравнивать не с чем, решает CHECK в БД.
	if minThreshold == nil || maxThreshold == nil {
		return
	}

	if *minThreshold >= *maxThreshold {
		sl.ReportError(minThreshold, "min_threshold", "MinThreshold", "ltfield", "max_threshold")
	}
}
