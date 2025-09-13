package di

import (
	"net/http"
	"smart_schedule_parser/internal/config"
	"smart_schedule_parser/internal/handlers"
)

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

	mux := http.NewServeMux()
	handlers := handlers.NewHandlers(mux)
	handlers.RegisterHandlers()

	return &Container{
		Config: cfg,
		Mux:    mux,
	}, nil
}
