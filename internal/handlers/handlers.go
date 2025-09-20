// Package handlers содержит обработчики HTTP-запросов.
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"smart_schedule_parser/internal/config"

	"smart_schedule_parser/internal/crawler"
)

// NewHandlers создает новый экземпляр Handlers с переданным ServeMux.
func NewHandlers(mux *http.ServeMux, crawler crawler.Crawler, config config.Config) *Handlers {
	return &Handlers{
		Mux:     mux,
		Crawler: crawler,
		Config:  config,
	}
}

// Handlers представляет собой набор HTTP-обработчиков, связанных с ServeMux.
type Handlers struct {
	Mux     *http.ServeMux
	Crawler crawler.Crawler
	Config  config.Config
}

// RegisterHandlers регистрирует обработчики для маршрутов
func (h *Handlers) RegisterHandlers() {
	// Здесь можно зарегистрировать свои обработчики
	h.Mux.HandleFunc(GetSchedule, h.getScheduleHandler)
}

func (h *Handlers) getScheduleHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	urlParam := r.URL.Query().Get("urlSchedule")
	if urlParam == "" {
		http.Error(w, "Get-параметр 'urlSchedule' пустой или его не существует", http.StatusBadRequest)
		return
	}

	links, err := h.Crawler.CrawlPages(ctx, "https://"+urlParam, h.Config.OutputDir)
	if err != nil {
		fmt.Println("Error crawling pages:", err)
	}
	fmt.Println(len(links))
	for key, link := range links {
		fmt.Printf("%d: %s\n", key, link)
	}
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte("OK"))
	if err != nil {
		return
	}
}
