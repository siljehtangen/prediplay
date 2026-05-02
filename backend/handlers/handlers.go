package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"prediplay/backend/bzzoiro"
	"prediplay/backend/services"

	"github.com/go-chi/chi/v5"
)

// Handler wires together the bzzoiro API client and the prediction service
// to serve all HTTP endpoints.
type Handler struct {
	bzzoiro    *bzzoiro.Client
	prediction *services.PredictionService
}

// New creates a Handler backed by the given bzzoiro client and prediction service.
func New(b *bzzoiro.Client, p *services.PredictionService) *Handler {
	return &Handler{bzzoiro: b, prediction: p}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func timeFilterParam(r *http.Request) string {
	if f := r.URL.Query().Get("time_filter"); f != "" {
		return f
	}
	return "recent"
}

func parsePlayerID(r *http.Request) (uint, bool) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(id), true
}

func normalizeLeagueName(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "premier league") || strings.Contains(n, "premier ligue"):
		return "Premier League"
	case strings.Contains(n, "la liga") || strings.Contains(n, "primera division"):
		return "La Liga"
	case strings.Contains(n, "bundesliga"):
		return "Bundesliga"
	case strings.Contains(n, "serie a"):
		return "Serie A"
	case strings.Contains(n, "ligue 1") || strings.Contains(n, "ligue1"):
		return "Ligue 1"
	}
	return ""
}
