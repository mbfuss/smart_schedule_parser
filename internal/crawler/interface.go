package crawler

import "context"

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
