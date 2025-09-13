package provider

import (
	"context"
	"smart_schedule_parser/internal/resource"
)

// Provider — интерфейс для провайдера расписаний (парсера).
type Provider interface {
	GetBuilding(ctx context.Context) ([]resource.Building, error)
}
