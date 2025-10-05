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

	zerolog "github.com/rs/zerolog/log"
)

// Provider — интерфейс для провайдера расписаний (парсера).
type Provider interface {
	// Получает список корпусов, парся PDF-файлы, скачанные по urlParam в outputDir.
	GetBuilding(ctx context.Context, urlParam string, outputDir string) ([]resource.Building, error)
}

// Service — реализация интерфейса Provider.
type Service struct {
	Crawler crawler.Crawler // Кравлер для скачивания файлов
	Parser  parser.Parser   // Парсер для обработки PDF-файлов
}

// NewProvider создает новый экземпляр Provider.
func NewProvider(crawler crawler.Crawler, parser parser.Parser) *Service {
	// Возвращает новый сервис с внедрёнными зависимостями кравлера и парсера
	return &Service{
		Crawler: crawler,
		Parser:  parser,
	}
}

// GetBuilding инициирует основной процесс получения и парсинга расписаний.
// Также группирует полученные группы по корпусам, институтам и формам обучения.
func (s *Service) GetBuilding(ctx context.Context, urlParam string, outputDir string) ([]resource.Building, error) {
	err := s.clearDir(outputDir)
	if err != nil {
		return nil, fmt.Errorf("очистка директории %s: %w", outputDir, err)
	}

	// Скачиваем страницы с помощью кравлера по переданному urlParam
	err = s.Crawler.CrawlPages(ctx, "https://"+urlParam, outputDir)
	if err != nil {
		return nil, fmt.Errorf("при обращении к кравлеру: %w", err)
	}

	buildingMap := make(map[string]*resource.Building) // Карта для группировки корпусов по имени
	var allGroups []resource.Group                     // Список всех найденных групп
	// Рекурсивно обходим все файлы в выходной директории, ищем PDF-файлы
	err = filepath.WalkDir(outputDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("обхода файла %s: %w", path, err)
		}

		// Проверяем, что это файл (а не директория) и расширение .pdf
		if !d.IsDir() && filepath.Ext(path) == ".pdf" {
			// Получаем относительный путь файла относительно outputDir
			rel, err := filepath.Rel(outputDir, path)
			if err != nil {
				return err
			}
			// Разбиваем путь на части (корпус, институт, форма обучения)
			parts := s.splitPath(rel)
			if len(parts) < 3 {
				// Если путь не содержит нужного количества частей, пропускаем файл
				return nil
			}
			buildingName := parts[0]  // Имя корпуса
			instituteName := parts[1] // Имя института
			studyFormName := parts[2] // Форма обучения

			// Парсим PDF-файл, получаем группы
			groups, err := s.Parser.ParsePDF(ctx, path)
			if err != nil {
				zerolog.Error().Msgf("Ошибка парсинга PDF %s: %v\n", path, err)
				return nil
			}
			// Добавляем все группы в общий список
			for _, group := range groups {
				allGroups = append(allGroups, group)
			}

			// Группируем по корпусу
			b, ok := buildingMap[buildingName]
			if !ok {
				// Если корпуса ещё нет, создаём новый
				b = &resource.Building{Name: buildingName}
				buildingMap[buildingName] = b
			}
			// Ищем институт внутри корпуса
			var inst *resource.Institute
			for i := range b.Institutes {
				if b.Institutes[i].Name == instituteName {
					inst = &b.Institutes[i]
					break
				}
			}
			if inst == nil {
				// Если института нет, добавляем новый
				b.Institutes = append(b.Institutes, resource.Institute{Name: instituteName})
				inst = &b.Institutes[len(b.Institutes)-1]
			}

			// Ищем форму обучения внутри института
			var form *resource.StudyForm
			for i := range inst.Forms {
				if inst.Forms[i].Name == studyFormName {
					form = &inst.Forms[i]
					break
				}
			}
			if form == nil {
				// Если формы нет, добавляем новую
				inst.Forms = append(inst.Forms, resource.StudyForm{Name: studyFormName})
				form = &inst.Forms[len(inst.Forms)-1]
			}
			// Добавляем группы к форме обучения
			form.Groups = append(form.Groups, groups...)
		}
		return nil
	})

	// Если возникла ошибка при обходе файлов, возвращаем ошибку
	if err != nil {
		return nil, err
	}

	var buildings []resource.Building
	// Преобразуем карту корпусов в срез для возврата
	for _, b := range buildingMap {
		buildings = append(buildings, *b)
	}

	zerolog.Info().Msgf("Получено %d групп", len(allGroups))

	return buildings, nil
}

// splitPath — вспомогательная функция для разбивки пути на части.
func (s *Service) splitPath(path string) []string {
	var parts []string
	for {
		dir, file := filepath.Split(path)
		if file != "" {
			// Добавляем имя файла/директории в начало среза
			parts = append([]string{file}, parts...)
		}
		if dir == "" || dir == "/" || dir == "." {
			// Если дошли до корня, завершаем цикл
			break
		}
		path = filepath.Clean(dir)
	}
	return parts
}

// clearDir удаляет все содержимое директории dir.
func (s *Service) clearDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		entryPath := filepath.Join(dir, entry.Name())
		err = os.RemoveAll(entryPath)
		if err != nil {
			return err
		}
	}
	return nil
}
