package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/ChuTTy32/SimpleMQTTMonitoring/api/internal/model"
	"github.com/ChuTTy32/SimpleMQTTMonitoring/api/internal/repository/sqlc"
)

// SensorRepository — CRUD для sensors поверх сгенерированного sqlc-кода.
type SensorRepository struct {
	q *sqlc.Queries
}

func NewSensorRepository(db sqlc.DBTX) *SensorRepository {
	return &SensorRepository{q: sqlc.New(db)}
}

func (r *SensorRepository) List(ctx context.Context, filter model.SensorFilter, limit, offset int32) ([]model.Sensor, error) {
	rows, err := r.q.ListSensors(ctx, sqlc.ListSensorsParams{
		Limit:        limit,
		Offset:       offset,
		ControllerID: filter.ControllerID,
	})
	if err != nil {
		return nil, mapError(err)
	}

	// make(..., 0, len) — пустой результат должен стать [], а не null.
	out := make([]model.Sensor, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSensorModel(row))
	}
	return out, nil
}

func (r *SensorRepository) Count(ctx context.Context, filter model.SensorFilter) (int64, error) {
	n, err := r.q.CountSensors(ctx, filter.ControllerID)
	if err != nil {
		return 0, mapError(err)
	}
	return n, nil
}

func (r *SensorRepository) GetByID(ctx context.Context, id uuid.UUID) (model.Sensor, error) {
	row, err := r.q.GetSensor(ctx, id)
	if err != nil {
		return model.Sensor{}, mapError(err)
	}
	return toSensorModel(row), nil
}

func (r *SensorRepository) Create(ctx context.Context, in model.CreateSensorInput) (model.Sensor, error) {
	row, err := r.q.CreateSensor(ctx, sqlc.CreateSensorParams{
		ControllerID: in.ControllerID,
		Name:         in.Name,
		Topic:        in.Topic,
		MetricType:   in.MetricType,
		Unit:         in.Unit,
		MinThreshold: in.MinThreshold,
		MaxThreshold: in.MaxThreshold,
		IsActive:     in.IsActive,
	})
	if err != nil {
		return model.Sensor{}, mapError(err)
	}
	return toSensorModel(row), nil
}

func (r *SensorRepository) Update(ctx context.Context, id uuid.UUID, in model.UpdateSensorInput) (model.Sensor, error) {
	row, err := r.q.UpdateSensor(ctx, sqlc.UpdateSensorParams{
		ID:           id,
		ControllerID: in.ControllerID,
		Name:         in.Name,
		Topic:        in.Topic,
		MetricType:   in.MetricType,
		Unit:         in.Unit,
		MinThreshold: in.MinThreshold,
		MaxThreshold: in.MaxThreshold,
		IsActive:     in.IsActive,
	})
	if err != nil {
		return model.Sensor{}, mapError(err)
	}
	return toSensorModel(row), nil
}

func (r *SensorRepository) Delete(ctx context.Context, id uuid.UUID) error {
	affected, err := r.q.DeleteSensor(ctx, id)
	if err != nil {
		return mapError(err)
	}
	// DELETE несуществующей строки — не ошибка SQL, но для API это 404.
	if affected == 0 {
		return model.ErrNotFound
	}
	return nil
}

func toSensorModel(s sqlc.Sensor) model.Sensor {
	return model.Sensor{
		ID:           s.ID,
		ControllerID: s.ControllerID,
		Name:         s.Name,
		Topic:        s.Topic,
		MetricType:   s.MetricType,
		Unit:         s.Unit,
		MinThreshold: s.MinThreshold,
		MaxThreshold: s.MaxThreshold,
		// is_active в схеме nullable (BOOLEAN DEFAULT true), но NULL недостижим:
		// INSERT подставляет COALESCE(..., true), UPDATE сохраняет прежнее значение.
		IsActive:  deref(s.IsActive, true),
		CreatedAt: s.CreatedAt.Time,
		UpdatedAt: s.UpdatedAt.Time,
	}
}
