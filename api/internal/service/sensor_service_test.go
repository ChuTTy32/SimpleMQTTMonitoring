package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ChuTTy32/SimpleMQTTMonitoring/api/internal/model"
)

type fakeSensorRepo struct {
	listFn  func(ctx context.Context, f model.SensorFilter, limit, offset int32) ([]model.Sensor, error)
	countFn func(ctx context.Context, f model.SensorFilter) (int64, error)

	gotListFilter  model.SensorFilter
	gotCountFilter model.SensorFilter
}

func (f *fakeSensorRepo) List(ctx context.Context, filter model.SensorFilter, limit, offset int32) ([]model.Sensor, error) {
	f.gotListFilter = filter
	return f.listFn(ctx, filter, limit, offset)
}

func (f *fakeSensorRepo) Count(ctx context.Context, filter model.SensorFilter) (int64, error) {
	f.gotCountFilter = filter
	return f.countFn(ctx, filter)
}

func (f *fakeSensorRepo) GetByID(context.Context, uuid.UUID) (model.Sensor, error) {
	return model.Sensor{}, nil
}
func (f *fakeSensorRepo) Create(context.Context, model.CreateSensorInput) (model.Sensor, error) {
	return model.Sensor{}, nil
}
func (f *fakeSensorRepo) Update(context.Context, uuid.UUID, model.UpdateSensorInput) (model.Sensor, error) {
	return model.Sensor{}, nil
}
func (f *fakeSensorRepo) Delete(context.Context, uuid.UUID) error { return nil }

func TestSensorServiceList(t *testing.T) {
	t.Run("склеивает страницу и общее количество", func(t *testing.T) {
		repo := &fakeSensorRepo{
			listFn: func(context.Context, model.SensorFilter, int32, int32) ([]model.Sensor, error) {
				return []model.Sensor{{Name: "s1"}}, nil
			},
			countFn: func(context.Context, model.SensorFilter) (int64, error) { return 7, nil },
		}

		items, total, err := NewSensorService(repo).List(context.Background(), model.SensorFilter{}, 50, 0)

		require.NoError(t, err)
		assert.Len(t, items, 1)
		assert.Equal(t, int64(7), total)
	})

	t.Run("фильтр уходит в оба запроса — иначе total не совпадёт со списком", func(t *testing.T) {
		id := uuid.New()
		repo := &fakeSensorRepo{
			listFn: func(context.Context, model.SensorFilter, int32, int32) ([]model.Sensor, error) {
				return []model.Sensor{}, nil
			},
			countFn: func(context.Context, model.SensorFilter) (int64, error) { return 0, nil },
		}

		_, _, err := NewSensorService(repo).List(context.Background(), model.SensorFilter{ControllerID: &id}, 50, 0)

		require.NoError(t, err)
		require.NotNil(t, repo.gotListFilter.ControllerID)
		require.NotNil(t, repo.gotCountFilter.ControllerID, "Count без фильтра вернул бы количество всех датчиков")
		assert.Equal(t, id, *repo.gotCountFilter.ControllerID)
	})

	t.Run("ошибка Count не проглатывается", func(t *testing.T) {
		boom := errors.New("db is down")
		repo := &fakeSensorRepo{
			listFn: func(context.Context, model.SensorFilter, int32, int32) ([]model.Sensor, error) {
				return []model.Sensor{}, nil
			},
			countFn: func(context.Context, model.SensorFilter) (int64, error) { return 0, boom },
		}

		_, _, err := NewSensorService(repo).List(context.Background(), model.SensorFilter{}, 50, 0)

		assert.ErrorIs(t, err, boom)
	})
}
