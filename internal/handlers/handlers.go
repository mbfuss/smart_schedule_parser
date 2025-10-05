// Package handlers содержит обработчики HTTP-запросов.
package handlers

import (
	"encoding/json"
	"net/http"
	"smart_schedule_parser/internal/config"
	"smart_schedule_parser/internal/provider"

	"context"
)

// NewHandlers создает новый экземпляр Handlers с переданным ServeMux.
func NewHandlers(mux *http.ServeMux, provider provider.Provider, config config.Config) *Handlers {
	return &Handlers{
		Mux:      mux,
		Provider: provider,
		Config:   config,
	}
}

// Handlers представляет собой набор HTTP-обработчиков, связанных с ServeMux.
type Handlers struct {
	Mux      *http.ServeMux
	Provider provider.Provider
	Config   config.Config
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

	building, err := h.Provider.GetBuilding(ctx, urlParam, h.Config.OutputDir)
	if err != nil {
		http.Error(w, "Ошибка при получении расписания: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(building)
	if err != nil {
		http.Error(w, "Ошибка сериализации JSON: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
