// Package provider определяет интерфейсы провайдеров расписаний.
package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"smart_schedule_parser/internal/crawler"
	"smart_schedule_parser/internal/parser"
	"smart_schedule_parser/internal/resource"

	zerolog "github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
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
	start := time.Now()
	err := s.clearDir(outputDir)
	if err != nil {
		return nil, fmt.Errorf("очистка директории %s: %w", outputDir, err)
	}

	// Скачиваем страницы с помощью кравлера по переданному urlParam
	err = s.Crawler.CrawlPages(ctx, "https://"+urlParam, outputDir)
	if err != nil {
		return nil, fmt.Errorf("при обращении к кравлеру: %w", err)
	}

	parsingResults := make(chan *parsingResult)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	g, gCtx := errgroup.WithContext(ctx)

	// Запускаем парсинг PDF-файлов
	g.Go(s.parsePDFFunc(gCtx, outputDir, parsingResults))

	buildingMap := make(map[string]*resource.Building) // Карта для группировки корпусов по имени
	allGroups := new([]resource.Group)                 // Список всех найденных групп
	// Запускаем обработку результатов
	g.Go(s.processParsingResult(gCtx, parsingResults, allGroups, buildingMap))

	// Если возникла ошибка в горутинах, возвращаем ошибку
	if err = g.Wait(); err != nil {
		zerolog.Error().Err(err).Msgf("ожидание завершения горутин")
		return nil, fmt.Errorf("ожидание завершения горутин: %w", err)
	}

	var buildings []resource.Building
	// Преобразуем карту корпусов в срез для возврата
	for _, b := range buildingMap {
		buildings = append(buildings, *b)
	}

	zerolog.Info().Msgf("Получено %d групп, выполнено за %s", len(*allGroups), time.Since(start))

	return buildings, nil
}

func (s *Service) parsePDFFunc(ctx context.Context, outputDir string, parsingResults chan<- *parsingResult) func() (err error) {
	ctx, cancel := context.WithCancel(ctx)
	g, gCtx := errgroup.WithContext(ctx)
	return func() (err error) {
		defer func() {
			close(parsingResults)
			if panicErr := recover(); panicErr != nil {
				cancel() // Отмена контекста горутин парсинга
				err = fmt.Errorf("паника в горутине парсинга PDF: %v", panicErr)
			}
		}()
		// Рекурсивно обходим все файлы в выходной директории, ищем PDF-файлы
		err = filepath.WalkDir(outputDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("попытка получения доступа к %q: %w", path, err)
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
				g.Go(func() error {
					groups, err := s.Parser.ParsePDF(gCtx, path)
					select {
					case parsingResults <- &parsingResult{
						BuildingName:  buildingName,
						InstituteName: instituteName,
						StudyFormName: studyFormName,
						Path:          path,
						Groups:        groups,
						Err:           err,
					}:
					case <-gCtx.Done():
						return gCtx.Err()
					}
					return nil
				})
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("обход директории %q: %w", outputDir, err)
		}
		if err = g.Wait(); err != nil {
			return fmt.Errorf("обработка результатов парсинга: %w", err)
		}
		return nil
	}
}

func (s *Service) processParsingResult(ctx context.Context, parsingResults <-chan *parsingResult, allGroups *[]resource.Group, buildingMap map[string]*resource.Building) func() (err error) {
	return func() (err error) {
		defer func() {
			if panicErr := recover(); panicErr != nil {
				err = fmt.Errorf("паника в горутине обработки результатов парсинга: %v", panicErr)
			}
		}()
		for {
			select {
			case <-ctx.Done():
				// Если контекст отменён, завершаем цикл
				return fmt.Errorf("обработка результатов парсинга прервана: %w", ctx.Err())
			case result, ok := <-parsingResults:
				if !ok {
					// Канал закрыт — выходим
					return nil
				}
				if result.Err != nil {
					if !errors.Is(result.Err, context.Canceled) && !errors.Is(result.Err, context.DeadlineExceeded) {
						zerolog.Error().Msgf("Ошибка парсинга PDF %s: %v\n", result.Path, result.Err)
					}
					continue
				}

				// Добавляем все группы в общий список
				for _, group := range result.Groups {
					*allGroups = append(*allGroups, group)
				}

				// Группируем по корпусу
				b, ok := buildingMap[result.BuildingName]
				if !ok {
					// Если корпуса ещё нет, создаём новый
					b = &resource.Building{Name: result.BuildingName}
					buildingMap[result.BuildingName] = b
				}
				// Ищем институт внутри корпуса
				var inst *resource.Institute
				for i := range b.Institutes {
					if b.Institutes[i].Name == result.InstituteName {
						inst = &b.Institutes[i]
						break
					}
				}
				if inst == nil {
					// Если института нет, добавляем новый
					b.Institutes = append(b.Institutes, resource.Institute{Name: result.InstituteName})
					inst = &b.Institutes[len(b.Institutes)-1]
				}

				// Ищем форму обучения внутри института
				var form *resource.StudyForm
				for i := range inst.Forms {
					if inst.Forms[i].Name == result.StudyFormName {
						form = &inst.Forms[i]
						break
					}
				}
				if form == nil {
					// Если формы нет, добавляем новую
					inst.Forms = append(inst.Forms, resource.StudyForm{Name: result.StudyFormName})
					form = &inst.Forms[len(inst.Forms)-1]
				}
				// Добавляем группы к форме обучения
				form.Groups = append(form.Groups, result.Groups...)
			}
		}
	}
}

type parsingResult struct {
	BuildingName  string // Имя корпуса
	InstituteName string // Имя института
	StudyFormName string // Форма обучения
	Path          string // Путь к PDF-файлу
	Groups        []resource.Group
	Err           error
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
