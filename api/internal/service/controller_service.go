// Package service — бизнес-логика. Не знает ни про HTTP, ни про SQL: принимает доменные
// типы, возвращает доменные типы и sentinel-ошибки.
package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/ChuTTy32/SimpleMQTTMonitoring/api/internal/model"
)

// controllerRepo объявлен здесь, в потребителе, а не в пакете repository — интерфейс
// описывает то, что нужно ЭТОМУ сервису, а не всё, что умеет репозиторий. Побочный
// эффект: в тестах сервис подменяется заглушкой без обращения к БД.
type controllerRepo interface {
	List(ctx context.Context, limit, offset int32) ([]model.Controller, error)
	Count(ctx context.Context) (int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (model.Controller, error)
	Create(ctx context.Context, in model.CreateControllerInput) (model.Controller, error)
	Update(ctx context.Context, id uuid.UUID, in model.UpdateControllerInput) (model.Controller, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type ControllerService struct {
	repo controllerRepo
}

func NewControllerService(repo controllerRepo) *ControllerService {
	return &ControllerService{repo: repo}
}

// List возвращает страницу контроллеров и общее количество — total нужен фронту для
// постраничной навигации и не выводится из длины страницы.
func (s *ControllerService) List(ctx context.Context, limit, offset int32) ([]model.Controller, int64, error) {
	items, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.repo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (s *ControllerService) GetByID(ctx context.Context, id uuid.UUID) (model.Controller, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ControllerService) Create(ctx context.Context, in model.CreateControllerInput) (model.Controller, error) {
	return s.repo.Create(ctx, in)
}

func (s *ControllerService) Update(ctx context.Context, id uuid.UUID, in model.UpdateControllerInput) (model.Controller, error) {
	return s.repo.Update(ctx, id, in)
}

func (s *ControllerService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
