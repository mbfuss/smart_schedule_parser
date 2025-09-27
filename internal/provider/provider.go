// Package provider определяет интерфейсы провайдеров расписаний.
package provider

import (
	"context"
	"smart_schedule_parser/internal/crawler"
	"smart_schedule_parser/internal/parser"

	"smart_schedule_parser/internal/resource"
)

// Provider — интерфейс для провайдера расписаний (парсера).
type Provider interface {
	GetBuilding(ctx context.Context) ([]resource.Building, error)
}

// Service — реализация интерфейса Provider.
type Service struct {
	Crawler crawler.Crawler
	Parser  parser.Parser
}

// NewProvider создает новый экземпляр Provider.
func NewProvider(crawler crawler.Crawler, parser parser.Parser) *Service {
	return &Service{
		Crawler: crawler,
		Parser:  parser,
	}
}

func (s *Service) GetBuilding(ctx context.Context) ([]resource.Building, error) {
	return []resource.Building{}, nil
}
