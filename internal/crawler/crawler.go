// Package crawler предоставляет функции и интерфейсы для обхода веб-страниц
// и извлечения PDF-файлов и других данных расписаний.
package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// CrawlResult представляет результат обхода страниц: найденные PDF-ссылки и карту посещённых URL.
type CrawlResult struct {
	PDFLinks []string
	Visited  map[string]bool
}

// CrawlPages рекурсивно обходит страницы, начиная с baseURL, и собирает все PDF-ссылки.
func CrawlPages(ctx context.Context, baseURL string, visited map[string]bool) ([]string, error) {
	// Если карта посещённых страниц не передана — создаём новую.
	// Требуется для отслеживания посещённых страниц.
	if visited == nil {
		visited = make(map[string]bool)
	}
	// Если страницу уже посещали — выходим, чтобы не зациклиться.
	if visited[baseURL] {
		return nil, nil
	}
	visited[baseURL] = true

	// Загружаем HTML страницы.
	resp, err := http.Get(baseURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса %s: %w", baseURL, err)
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			err = fmt.Errorf("закрытие тела ответа: %w", err)
		}
	}(resp.Body)

	// Проверяем успешность ответа.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("не удалось получить страницу %s: %s", baseURL, resp.Status)
	}

	// Парсим HTML с помощью goquery.
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга HTML: %w", err)
	}

	var pdfLinks []string
	base, _ := url.Parse(baseURL)

	// Ищем все ссылки <a> на странице.
	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		link, err := url.Parse(href)
		if err != nil {
			return
		}
		absURL := base.ResolveReference(link).String()

		// Если ссылка на PDF — добавляем в результат.
		if strings.HasSuffix(strings.ToLower(absURL), ".pdf") {
			pdfLinks = append(pdfLinks, absURL)
		} else if strings.Contains(absURL, "raspisanie") && !visited[absURL] {
			// Если ссылка похожа на страницу с расписанием и ещё не посещалась —
			// рекурсивно обходим её.
			subLinks, err := CrawlPages(ctx, absURL, visited)
			if err == nil {
				pdfLinks = append(pdfLinks, subLinks...)
			}
		}
	})

	// Возвращаем все найденные PDF-ссылки.
	return pdfLinks, nil
}
