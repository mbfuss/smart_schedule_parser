// Package handlers содержит обработчики HTTP-запросов.
package handlers

import (
	"context"
	"fmt"
	"net/http"

	"smart_schedule_parser/internal/crawler"
)

// NewHandlers создает новый экземпляр Handlers с переданным ServeMux.
func NewHandlers(mux *http.ServeMux) *Handlers {
	return &Handlers{
		Mux: mux,
	}
}

// Handlers представляет собой набор HTTP-обработчиков, связанных с ServeMux.
type Handlers struct {
	Mux *http.ServeMux
}

// RegisterHandlers регистрирует обработчики для маршрутов
func (H *Handlers) RegisterHandlers() {
	// Здесь можно зарегистрировать свои обработчики
	H.Mux.HandleFunc(GetSchedule, getScheduleHandler)
}

func getScheduleHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	links, err := crawler.CrawlPages(ctx, "https://www.vavilovsar.ru/upravlenie-obespecheniya-kachestva-obrazovaniya/struktura/otdel-organizacii-uchebnogo-processa/uk1/institut-genetiki-i-agronomii/ochnaya-forma-obucheniya", nil)
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
