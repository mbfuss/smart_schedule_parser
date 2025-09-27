// Package provider определяет интерфейсы провайдеров расписаний.
package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"smart_schedule_parser/internal/crawler"
	"smart_schedule_parser/internal/parser"

	"smart_schedule_parser/internal/resource"
)

// Provider — интерфейс для провайдера расписаний (парсера).
type Provider interface {
	GetBuilding(ctx context.Context, urlParam string, outputDir string) ([]resource.Building, error)
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

func (s *Service) GetBuilding(ctx context.Context, urlParam string, outputDir string) ([]resource.Building, error) {
	//_, err := s.Crawler.CrawlPages(ctx, "https://"+urlParam, outputDir)
	//if err != nil {
	//	return nil, fmt.Errorf("при обращении к кравлеру: %w", err)
	//}

	var allGroups []resource.Group
	err := filepath.WalkDir(outputDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && filepath.Ext(path) == ".pdf" {
			groups, err := s.Parser.ParsePDF(ctx, fmt.Sprintf("123./"+path))
			if err != nil {
				// Можно залогировать ошибку, но не прерывать обход
				fmt.Printf("Ошибка парсинга PDF %s: %v\n", path, err)
				return nil
			}
			allGroups = append(allGroups, groups...)
		}
		return nil
	})
	fmt.Println(allGroups)
	if err != nil {
		return nil, err
	}

	// Здесь можно собрать структуру []resource.Building из allGroups
	return []resource.Building{}, nil
}
