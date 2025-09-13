package parser

import (
	"context"
	"smart_schedule_parser/internal/resource"
)

type Parser interface {
	// ParsePDF — парсит PDF-файл по пути filePath и возвращает массив групп с расписаниями.
	ParsePDF(ctx context.Context, filePath string) ([]resource.Group, error)
}
