// Package crawler предоставляет функции и интерфейсы для обхода веб-страниц
// и извлечения PDF-файлов и других данных расписаний.
package crawler

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
	zerolog "github.com/rs/zerolog/log"
)

const universityURL = "https://www.vavilovsar.ru"

// StudyForm — форма обучения (очная, заочная, очно-заочная).
// Key — ключ формы, например "och" или "zao".
// DisplayName — удобочитаемое имя.
type StudyForm struct {
	// Key    string  `json:"key"`              // внутренний ключ формы обучения
	Name string `json:"name"` // отображаемое имя формы обучения
}

// Institute — институт/факультет.
// Code — код института.
// Name — название института.
// Forms — формы обучения внутри института (о/зао).
type Institute struct {
	// Code  string            `json:"code"`            // код института (например "INF")
	Name  string      `json:"name"`            // полное название института
	Forms []StudyForm `json:"forms,omitempty"` // формы обучения внутри института
}

// Building — учебный корпус.
// Institutes — список институтов в корпусе.
type Building struct {
	Name       string      `json:"name"`                 // название корпуса
	Institutes []Institute `json:"institutes,omitempty"` // институты в корпусе
}

// Crawler — интерфейс для краулера (обходчика) страниц.
type Crawler interface {
	// CrawlPages обходит сайт с baseURL, возвращает корпуса
	// и сохраняет PDF-файлы по иерархии Building
	CrawlPages(ctx context.Context, baseURL string, outputDir string) ([]Building, error)
}

// Service — реализация интерфейса Crawler.
type Service struct {
	UKCount            int // Счетчик количества найденных корпусов
	InstituteCount     int // Счетчик количества найденных институтов
	StudyFormCount     int // Счетчик количества найденных форм обучения
	PdfCountAll        int // Счетчик количества найденных PDF-файлов
	PdfCountDownloaded int // Счетчик количества успешно загруженных PDF-файлов
}

// NewCrawler создает новый экземпляр CrawlerImpl.
func NewCrawler() *Service {
	return &Service{}
}

// CrawlPages — основной оркестратор обхода сайта и скачивания PDF
func (s *Service) CrawlPages(ctx context.Context, baseURL string, outputDir string) ([]Building, error) {
	zerolog.Info().Msgf("Начало выполнения crawler")
	buildings, err := s.parseBuildings(ctx, baseURL, outputDir)
	if err != nil {
		return nil, err
	}
	s.logStats()
	s.resetCounters()
	return buildings, nil
}

// parseBuildings — парсит корпуса и институты на baseURL
func (s *Service) parseBuildings(ctx context.Context, baseURL, outputDir string) ([]Building, error) {
	var buildings []Building
	doc, err := fetchDocument(baseURL)
	if err != nil {
		return nil, err
	}
	doc.Find("header.header").Remove()
	doc.Find("footer.footer").Remove()
	doc.Find("a").Each(func(_ int, h *goquery.Selection) {
		href, exists := h.Attr("href")
		if exists && s.isUkPage(href) && s.isInstitutePage(href) {
			buildingName := s.extractBuildingName(href)
			instituteName := s.extractInstituteName(href)
			building := s.getOrCreateBuilding(&buildings, buildingName)
			institute := s.getOrCreateInstitute(building, instituteName)
			s.parseInstitutes(ctx, href, building, institute, outputDir)
		}
	})
	return buildings, nil
}

// parseInstitutes — парсит содержимое институтов внутри корпуса
func (s *Service) parseInstitutes(ctx context.Context, href string, building *Building, institute *Institute, outputDir string) {
	doc, err := fetchDocument(universityURL + href)
	if err != nil {
		zerolog.Error().Err(err).Msgf("Ошибка парсинга института: %s", href)
		return
	}
	doc.Find("header.header").Remove()
	doc.Find("ul.breadcrumb__list").Remove()
	doc.Find("footer.footer").Remove()
	doc.Find("a").Each(func(_ int, h *goquery.Selection) {
		formHref, exists := h.Attr("href")
		if exists && strings.Contains(formHref, "forma-obucheniya") {
			formName := s.extractFormName(formHref)
			// Получаем или создаем форму обучения
			form := s.getOrCreateForm(institute, formName)
			s.StudyFormCount++
			s.parseStudyForms(ctx, formHref, building, institute, form, outputDir)
		}
	})
}

// parseStudyForms — парсит содержимое форм обучения, ищет PDF и скачивает их
func (s *Service) parseStudyForms(ctx context.Context, formHref string, building *Building, institute *Institute, form *StudyForm, outputDir string) {
	doc, err := fetchDocument(formHref)
	if err != nil {
		zerolog.Error().Err(err).Msgf("Ошибка парсинга формы: %s", formHref)
		return
	}
	doc.Find("header.header").Remove()
	doc.Find("ul.breadcrumb__list").Remove()
	doc.Find("footer.footer").Remove()
	doc.Find("a").Each(func(_ int, h *goquery.Selection) {
		pdfHref, exists := h.Attr("href")
		if exists && strings.HasSuffix(pdfHref, ".pdf") {
			s.downloadAndSavePDF(ctx, pdfHref, building.Name, institute.Name, form.Name, outputDir)
		}
	})
}

// downloadAndSavePDF — скачивает PDF и сохраняет по иерархии
func (s *Service) downloadAndSavePDF(ctx context.Context, pdfHref, buildingName, instituteName, formName, outputDir string) {
	fileLink := universityURL + pdfHref
	dirPath := filepath.Join(outputDir, buildingName, instituteName, formName)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		zerolog.Error().Err(err).Msgf("Ошибка создания директории: %s", dirPath)
		return
	}
	fileName := filepath.Base(pdfHref)
	filePath := filepath.Join(dirPath, fileName)
	err := s.downloadFile(ctx, fileLink, filePath)
	s.PdfCountAll++
	if err != nil {
		zerolog.Error().Err(err).Msgf("Ошибка загрузки файла: %s", fileLink)
	} else {
		s.PdfCountDownloaded++
	}
}

// fetchDocument — получает и парсит HTML-документ по URL
func fetchDocument(url string) (*goquery.Document, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			zerolog.Error().Err(err).Msg("Ошибка при закрытии resp.Body")
		}
	}()
	return goquery.NewDocumentFromReader(resp.Body)
}

// getOrCreateBuilding — ищет или создает корпус
func (s *Service) getOrCreateBuilding(buildings *[]Building, name string) *Building {
	for i := range *buildings {
		if (*buildings)[i].Name == name {
			return &(*buildings)[i]
		}
	}
	*buildings = append(*buildings, Building{Name: name})
	s.UKCount++
	return &(*buildings)[len(*buildings)-1]
}

// getOrCreateInstitute — ищет или создает институт
func (s *Service) getOrCreateInstitute(building *Building, name string) *Institute {
	for i := range building.Institutes {
		if building.Institutes[i].Name == name {
			return &building.Institutes[i]
		}
	}
	building.Institutes = append(building.Institutes, Institute{Name: name})
	s.InstituteCount++
	return &building.Institutes[len(building.Institutes)-1]
}

// getOrCreateForm — ищет или создает форму обучения
func (s *Service) getOrCreateForm(institute *Institute, name string) *StudyForm {
	for i := range institute.Forms {
		if institute.Forms[i].Name == name {
			return &institute.Forms[i]
		}
	}
	institute.Forms = append(institute.Forms, StudyForm{Name: name})
	return &institute.Forms[len(institute.Forms)-1]
}

// logStats — выводит статистику по найденным объектам
func (s *Service) logStats() {
	zerolog.Info().Msgf("Найдено учебных корпусов: %v", s.UKCount)
	zerolog.Info().Msgf("Найдено институтов: %v", s.InstituteCount)
	zerolog.Info().Msgf("Найдено форм обучения: %v", s.StudyFormCount)
	zerolog.Info().Msgf("Найдено всего PDF файлов: %v", s.PdfCountAll)
	zerolog.Info().Msgf("Cкачано PDF файлов: %v", s.PdfCountDownloaded)
	zerolog.Info().Msgf("Выполнение crawler завершено")
}

// isUkPage проверяет, является ли ссылка ссылкой на страницу корпуса (uk1, uk2, uk3).
func (s *Service) isUkPage(href string) bool {
	return strings.Contains(href, "/uk")
}

// extractBuildingName извлекает название корпуса из ссылки.
func (s *Service) extractBuildingName(href string) string {
	parts := strings.Split(href, "/")
	for _, p := range parts {
		if strings.HasPrefix(p, "uk") {
			return p
		}
	}
	return "unknown"
}

// isInstitutePage проверяет, является ли ссылка ссылкой на страницу института.
func (s *Service) isInstitutePage(href string) bool {
	return strings.Contains(href, "/institut")
}

// extractBuildingName извлекает название корпуса из ссылки.
func (s *Service) extractInstituteName(href string) string {
	parts := strings.Split(href, "/")
	for _, p := range parts {
		if strings.HasPrefix(p, "institut") {
			return p
		}
	}
	return "unknown"
}

// extractFormName извлекает названия формы обучения
func (s *Service) extractFormName(href string) string {
	parts := strings.Split(href, "/")
	for _, p := range parts {
		if strings.Contains(p, "forma-obucheniya") {
			return strings.ReplaceAll(p, "-", " ")
		}
	}
	return "unknown"
}

// Скачивает файл по url в указанный путь
func (s *Service) downloadFile(ctx context.Context, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			zerolog.Error().Err(err).Msg("Ошибка при закрытии resp.Body")
		}
	}()

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		err = out.Close()
		if err != nil {
			zerolog.Error().Err(err).Msg("Ошибка при закрытии os.Create")
		}
	}()

	_, err = io.Copy(out, resp.Body)
	return err
}

// resetCounters — сбрасывает все счетчики Service
func (s *Service) resetCounters() {
	s.UKCount = 0
	s.InstituteCount = 0
	s.StudyFormCount = 0
	s.PdfCountAll = 0
	s.PdfCountDownloaded = 0
}
