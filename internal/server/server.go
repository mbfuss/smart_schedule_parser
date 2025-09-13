package server

import (
	"fmt"
	"net/http"
	"smart_schedule_parser/internal/config"
)

// StartServer запускает HTTP сервер
func StartServer(mux *http.ServeMux, cfg *config.Config) error {
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	err := http.ListenAndServe(addr, mux)
	if err != nil {
		panic(err)
	}
	return nil
}
