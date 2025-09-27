// Package provider определяет интерфейсы провайдеров расписаний.
package provider

import (
	"context"
	"smart_schedule_parser/internal/crawler"

	"smart_schedule_parser/internal/resource"
)

// Provider — интерфейс для провайдера расписаний (парсера).
type Provider interface {
	GetBuilding(ctx context.Context) ([]resource.Building, error)
}

// Service — реализация интерфейса Provider.
type Service struct {
	Crawler crawler.Crawler
}

// NewProvider создает новый экземпляр Provider.
func NewProvider(crawler crawler.Crawler) *Service {
	return &Service{
		Crawler: crawler,
	}
}

func (s *Service) GetBuilding(ctx context.Context) ([]resource.Building, error) {
	return []resource.Building{}, nil
}
