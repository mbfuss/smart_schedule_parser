// Package di предоставляет контейнер зависимостей для приложения,
// включая конфигурацию и HTTP-маршрутизатор.
package di

import (
	"net/http"
	"smart_schedule_parser/internal/crawler"
	"smart_schedule_parser/internal/provider"

	"smart_schedule_parser/internal/config"
	"smart_schedule_parser/internal/handlers"

	zerolog "github.com/rs/zerolog/log"
)

// Container хранит зависимости приложения, включая конфигурацию и HTTP-маршрутизатор.
type Container struct {
	Config *config.Config
	Mux    *http.ServeMux
}

// NewContainer создает новый контейнер зависимостей
func NewContainer() (*Container, error) {
	// Загружаем конфиг
	if err := config.Load(); err != nil {
		return nil, err
	}
	cfg := config.GetConfig()
	zerolog.Info().Msgf("Конфигурация загружена: %+v", cfg)

	mux := http.NewServeMux()
	crawler := crawler.NewCrawler()
	provider := provider.NewProvider(crawler)
	handlers := handlers.NewHandlers(mux, provider, *cfg)
	handlers.RegisterHandlers()

	return &Container{
		Config: cfg,
		Mux:    mux,
	}, nil
}
