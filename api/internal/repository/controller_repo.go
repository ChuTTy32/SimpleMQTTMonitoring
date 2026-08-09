// Package repository — доступ к БД. Единственный слой, знающий про pgx и sqlc: наружу
// отдаёт доменные типы из model и sentinel-ошибки, а не типы драйвера.
package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ChuTTy32/SimpleMQTTMonitoring/api/internal/model"
	"github.com/ChuTTy32/SimpleMQTTMonitoring/api/internal/repository/sqlc"
)

// pgUniqueViolation — код ошибки PostgreSQL при нарушении UNIQUE-ограничения.
const pgUniqueViolation = "23505"

// ControllerRepository — CRUD для controllers поверх сгенерированного sqlc-кода.
type ControllerRepository struct {
	q *sqlc.Queries
}

func NewControllerRepository(db sqlc.DBTX) *ControllerRepository {
	return &ControllerRepository{q: sqlc.New(db)}
}

func (r *ControllerRepository) List(ctx context.Context, limit, offset int32) ([]model.Controller, error) {
	rows, err := r.q.ListControllers(ctx, sqlc.ListControllersParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, mapError(err)
	}

	// Именно make(..., 0, len) а не var slice: пустой результат должен сериализоваться
	// в [], а не в null — иначе фронту приходится обрабатывать оба случая.
	out := make([]model.Controller, 0, len(rows))
	for _, row := range rows {
		out = append(out, toModel(row))
	}
	return out, nil
}

func (r *ControllerRepository) Count(ctx context.Context) (int64, error) {
	n, err := r.q.CountControllers(ctx)
	if err != nil {
		return 0, mapError(err)
	}
	return n, nil
}

func (r *ControllerRepository) GetByID(ctx context.Context, id uuid.UUID) (model.Controller, error) {
	row, err := r.q.GetController(ctx, id)
	if err != nil {
		return model.Controller{}, mapError(err)
	}
	return toModel(row), nil
}

func (r *ControllerRepository) Create(ctx context.Context, in model.CreateControllerInput) (model.Controller, error) {
	row, err := r.q.CreateController(ctx, sqlc.CreateControllerParams{
		Name:          in.Name,
		MqttGatewayID: in.MQTTGatewayID,
		IpAddress:     in.IPAddress,
		Location:      in.Location,
		IsActive:      in.IsActive,
	})
	if err != nil {
		return model.Controller{}, mapError(err)
	}
	return toModel(row), nil
}

func (r *ControllerRepository) Update(ctx context.Context, id uuid.UUID, in model.UpdateControllerInput) (model.Controller, error) {
	row, err := r.q.UpdateController(ctx, sqlc.UpdateControllerParams{
		ID:            id,
		Name:          in.Name,
		MqttGatewayID: in.MQTTGatewayID,
		IpAddress:     in.IPAddress,
		Location:      in.Location,
		IsActive:      in.IsActive,
	})
	if err != nil {
		return model.Controller{}, mapError(err)
	}
	return toModel(row), nil
}

func (r *ControllerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	affected, err := r.q.DeleteController(ctx, id)
	if err != nil {
		return mapError(err)
	}
	// DELETE несуществующей строки — не ошибка на уровне SQL, но для API это 404.
	if affected == 0 {
		return model.ErrNotFound
	}
	return nil
}

// toModel переводит строку sqlc в доменный тип, вычищая типы драйвера
// (pgtype.Timestamptz) и nullable-обёртки там, где схема гарантирует значение.
func toModel(c sqlc.Controller) model.Controller {
	return model.Controller{
		ID:            c.ID,
		Name:          c.Name,
		MQTTGatewayID: c.MqttGatewayID,
		IPAddress:     c.IpAddress,
		Location:      c.Location,
		// is_active в схеме nullable (BOOLEAN DEFAULT true), но NULL недостижим:
		// INSERT всегда подставляет COALESCE(..., true), UPDATE сохраняет прежнее.
		// Дефолт true повторяет дефолт колонки.
		IsActive:  deref(c.IsActive, true),
		CreatedAt: c.CreatedAt.Time,
		UpdatedAt: c.UpdatedAt.Time,
	}
}

func deref[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}

// mapError переводит ошибки драйвера в доменные sentinel-ошибки. Всё, что не распознано,
// пробрасывается как есть — выше по стеку станет 500.
func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return model.ErrDuplicate
	}

	return err
}
