package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-playground/validator/v10"

	"github.com/ChuTTy32/SimpleMQTTMonitoring/api/internal/model"
)

// Коды ошибок — стабильный контракт для фронта. Текст message может меняться,
// code — нет: обработка на клиенте переключается по нему, а не по строке сообщения.
// См. docs/api-architecture.md, раздел «Формат ошибок».
const (
	codeValidation   = "validation_error"
	codeUnauthorized = "unauthorized"
	codeForbidden    = "forbidden"
	codeNotFound     = "not_found"
	codeDuplicate    = "duplicate"
	codeInternal     = "internal"
)

// errorStatus — единственное место, где code сопоставляется с HTTP-статусом.
var errorStatus = map[string]int{
	codeValidation:   http.StatusBadRequest,
	codeUnauthorized: http.StatusUnauthorized,
	codeForbidden:    http.StatusForbidden,
	codeNotFound:     http.StatusNotFound,
	codeDuplicate:    http.StatusConflict,
	codeInternal:     http.StatusInternalServerError,
}

type errorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

// listEnvelope — обёртка списочных ответов. Голый массив не даёт места под total,
// а он нужен для постраничной навигации.
type listEnvelope[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// Заголовки и статус уже ушли клиенту — исправить ответ нельзя, остаётся залогировать.
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to write response", "error", err)
	}
}

func writeError(w http.ResponseWriter, code, message string, fields map[string]string) {
	status, ok := errorStatus[code]
	if !ok {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, errorEnvelope{Error: errorBody{Code: code, Message: message, Fields: fields}})
}

// writeServiceError переводит доменные ошибки в ответ. Неизвестная ошибка — это баг или
// отказ инфраструктуры: клиенту уходит обезличенное «internal», подробности только в лог,
// чтобы наружу не утекали детали БД.
func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, model.ErrNotFound):
		writeError(w, codeNotFound, "resource not found", nil)
	case errors.Is(err, model.ErrDuplicate):
		writeError(w, codeDuplicate, "resource already exists", nil)
	// Ссылка в пустоту и нарушение CHECK — ошибки клиента, а не сервера: тело запроса
	// синтаксически корректно, но противоречит состоянию/правилам БД. 400, не 500.
	case errors.Is(err, model.ErrReferenceNotFound):
		writeError(w, codeValidation, "referenced resource does not exist", nil)
	case errors.Is(err, model.ErrConstraintViolation):
		writeError(w, codeValidation, "value violates a database constraint", nil)
	default:
		slog.Error("unhandled service error", "error", err)
		writeError(w, codeInternal, "internal server error", nil)
	}
}

// writeValidationError разворачивает ошибки validator в map{поле: причина}, чтобы форма
// на фронте подсветила конкретные поля, а не показала одну общую строку.
func writeValidationError(w http.ResponseWriter, err error) {
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		writeError(w, codeValidation, err.Error(), nil)
		return
	}

	fields := make(map[string]string, len(verrs))
	for _, fe := range verrs {
		fields[fe.Field()] = "failed rule: " + fe.Tag()
	}
	writeError(w, codeValidation, "validation failed", fields)
}
