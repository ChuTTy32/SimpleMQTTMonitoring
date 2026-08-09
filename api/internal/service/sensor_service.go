package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/ChuTTy32/SimpleMQTTMonitoring/api/internal/model"
)

// sensorRepo объявлен здесь, в потребителе — интерфейс описывает то, что нужно этому
// сервису. Фильтр принимают и List, и Count: иначе total в ответе считался бы по всем
// датчикам и расходился бы с отфильтрованной страницей.
type sensorRepo interface {
	List(ctx context.Context, filter model.SensorFilter, limit, offset int32) ([]model.Sensor, error)
	Count(ctx context.Context, filter model.SensorFilter) (int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (model.Sensor, error)
	Create(ctx context.Context, in model.CreateSensorInput) (model.Sensor, error)
	Update(ctx context.Context, id uuid.UUID, in model.UpdateSensorInput) (model.Sensor, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type SensorService struct {
	repo sensorRepo
}

func NewSensorService(repo sensorRepo) *SensorService {
	return &SensorService{repo: repo}
}

func (s *SensorService) List(ctx context.Context, filter model.SensorFilter, limit, offset int32) ([]model.Sensor, int64, error) {
	items, err := s.repo.List(ctx, filter, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.repo.Count(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (s *SensorService) GetByID(ctx context.Context, id uuid.UUID) (model.Sensor, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *SensorService) Create(ctx context.Context, in model.CreateSensorInput) (model.Sensor, error) {
	return s.repo.Create(ctx, in)
}

func (s *SensorService) Update(ctx context.Context, id uuid.UUID, in model.UpdateSensorInput) (model.Sensor, error) {
	return s.repo.Update(ctx, id, in)
}

func (s *SensorService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
