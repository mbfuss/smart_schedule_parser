package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"smart_schedule_parser/internal/config"
	"smart_schedule_parser/internal/provider"
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
	h.Mux.HandleFunc(GetSchedule, h.getScheduleHandler)
	h.Mux.HandleFunc(Health, h.healthHandler)
}

func (h *Handlers) healthHandler(w http.ResponseWriter, r *http.Request) {
	type resp struct {
		Status    string `json:"status"`
		OutputDir string `json:"output_dir"`
	}

	if _, err := os.Stat(h.Config.OutputDir); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(resp{
			Status:    "degraded",
			OutputDir: h.Config.OutputDir,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp{
		Status:    "ok",
		OutputDir: h.Config.OutputDir,
	})
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
