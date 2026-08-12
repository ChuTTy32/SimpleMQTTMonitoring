package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/ChuTTy32/SimpleMQTTMonitoring/api/internal/model"
)

// Пагинация: см. docs/api-architecture.md, раздел «Пагинация». limit сверх максимума не
// ошибка, а тихий clamp — фронту не нужно знать про лимит, чтобы не получить 400.
const (
	defaultLimit = 50
	maxLimit     = 200
)

// controllerService — интерфейс объявлен в потребителе (см. тот же приём в service/).
type controllerService interface {
	List(ctx context.Context, limit, offset int32) ([]model.Controller, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (model.Controller, error)
	Create(ctx context.Context, in model.CreateControllerInput) (model.Controller, error)
	Update(ctx context.Context, id uuid.UUID, in model.UpdateControllerInput) (model.Controller, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type ControllerHandler struct {
	svc      controllerService
	validate *validator.Validate
}

func NewControllerHandler(svc controllerService) *ControllerHandler {
	return &ControllerHandler{svc: svc, validate: newValidator()}
}

// Routes возвращает под-роутер модуля — router.go монтирует его под /controllers,
// не зная про внутренние пути.
func (h *ControllerHandler) Routes() chi.Router {
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

func (h *ControllerHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	items, total, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, listEnvelope[model.Controller]{Items: items, Total: total})
}

func (h *ControllerHandler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	c, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, c)
}

func (h *ControllerHandler) create(w http.ResponseWriter, r *http.Request) {
	var in model.CreateControllerInput
	if !decodeAndValidate(w, r, h.validate, &in) {
		return
	}

	c, err := h.svc.Create(r.Context(), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, c)
}

func (h *ControllerHandler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	var in model.UpdateControllerInput
	if !decodeAndValidate(w, r, h.validate, &in) {
		return
	}

	c, err := h.svc.Update(r.Context(), id, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, c)
}

func (h *ControllerHandler) delete(w http.ResponseWriter, r *http.Request) {
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

// parseID достаёт и валидирует {id} из пути. Возвращает ok=false, если ответ клиенту
// уже записан — вызывающему остаётся только выйти.
func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, codeValidation, "invalid id: expected uuid", nil)
		return uuid.UUID{}, false
	}
	return id, true
}

// parsePagination разбирает ?limit=&offset=. Некорректные значения не отвергаются, а
// заменяются дефолтами — параметры пагинации не та вещь, ради которой стоит ломать запрос.
func parsePagination(r *http.Request) (limit, offset int32) {
	limit = defaultLimit
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		if v > maxLimit {
			v = maxLimit
		}
		limit = int32(v)
	}

	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v > 0 {
		offset = int32(v)
	}

	return limit, offset
}

// decodeAndValidate — общий вход для тел запросов: разбор JSON + проверка правил
// validator. Возвращает false, если ответ об ошибке уже отправлен.
func decodeAndValidate(w http.ResponseWriter, r *http.Request, v *validator.Validate, dst any) bool {
	dec := json.NewDecoder(r.Body)
	// Неизвестное поле — почти всегда опечатка на клиенте. Молча его проглотить хуже,
	// чем сразу сказать: иначе «поле не сохраняется» ищут часами.
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		writeError(w, codeValidation, "invalid json body: "+err.Error(), nil)
		return false
	}

	if err := v.Struct(dst); err != nil {
		writeValidationError(w, err)
		return false
	}

	return true
}
