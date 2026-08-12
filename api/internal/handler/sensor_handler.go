package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/ChuTTy32/SimpleMQTTMonitoring/api/internal/model"
)

// sensorService — интерфейс объявлен в потребителе (тот же приём, что в service/).
type sensorService interface {
	List(ctx context.Context, filter model.SensorFilter, limit, offset int32) ([]model.Sensor, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (model.Sensor, error)
	Create(ctx context.Context, in model.CreateSensorInput) (model.Sensor, error)
	Update(ctx context.Context, id uuid.UUID, in model.UpdateSensorInput) (model.Sensor, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type SensorHandler struct {
	svc      sensorService
	validate *validator.Validate
}

func NewSensorHandler(svc sensorService) *SensorHandler {
	return &SensorHandler{svc: svc, validate: newValidator()}
}

func (h *SensorHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.list)
	r.Get("/{id}", h.get)

	// TODO(auth): мутирующие ручки должны быть под middleware.RequireRole("operator", "admin")
	// — см. таблицу маршрутов в docs/api-architecture.md. Сейчас открыты: middleware/auth.go
	// и requirerole.go в зоне друга (модуль auth) и ещё не написаны.
	r.Post("/", h.create)
	r.Patch("/{id}", h.update)
	r.Delete("/{id}", h.delete)

	return r
}

func (h *SensorHandler) list(w http.ResponseWriter, r *http.Request) {
	filter, ok := parseSensorFilter(w, r)
	if !ok {
		return
	}

	limit, offset := parsePagination(r)

	items, total, err := h.svc.List(r.Context(), filter, limit, offset)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, listEnvelope[model.Sensor]{Items: items, Total: total})
}

func (h *SensorHandler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	s, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, s)
}

func (h *SensorHandler) create(w http.ResponseWriter, r *http.Request) {
	var in model.CreateSensorInput
	if !decodeAndValidate(w, r, h.validate, &in) {
		return
	}

	s, err := h.svc.Create(r.Context(), in)
	if err != nil {
		writeSensorServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, s)
}

func (h *SensorHandler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	var in model.UpdateSensorInput
	if !decodeAndValidate(w, r, h.validate, &in) {
		return
	}

	s, err := h.svc.Update(r.Context(), id, in)
	if err != nil {
		writeSensorServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, s)
}

func (h *SensorHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// parseSensorFilter разбирает необязательный ?controller_id=. Мусор вместо uuid — это
// ошибка клиента (400), а не повод молча отдать список всех датчиков: тихое игнорирование
// битого фильтра выглядит на фронте как «фильтр не работает».
func parseSensorFilter(w http.ResponseWriter, r *http.Request) (model.SensorFilter, bool) {
	raw := r.URL.Query().Get("controller_id")
	if raw == "" {
		return model.SensorFilter{}, true
	}

	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, codeValidation, "invalid controller_id: expected uuid", nil)
		return model.SensorFilter{}, false
	}

	return model.SensorFilter{ControllerID: &id}, true
}

// writeSensorServiceError уточняет общий обработчик: для нарушения внешнего ключа
// указывает конкретное поле, чтобы форма на фронте подсветила именно controller_id.
func writeSensorServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, model.ErrReferenceNotFound) {
		writeError(w, codeValidation, "referenced controller does not exist",
			map[string]string{"controller_id": "no controller with this id"})
		return
	}

	writeServiceError(w, err)
}
