package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"prediplay/backend/bzzoiro"
	"prediplay/backend/models"
	"prediplay/backend/services"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	bzzoiro    *bzzoiro.Client
	prediction *services.PredictionService
}

func New(b *bzzoiro.Client, p *services.PredictionService) *Handler {
	return &Handler{bzzoiro: b, prediction: p}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func parseWeights(r *http.Request) models.PredictionWeights {
	w := models.DefaultWeights()
	raw := r.URL.Query().Get("weights")
	if raw == "" {
		return w
	}
	for _, part := range strings.Split(raw, ",") {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(kv[1]), 64)
		if err != nil {
			continue
		}
		switch strings.TrimSpace(kv[0]) {
		case "form":
			w.Form = v
		case "threat":
			w.Threat = v
		case "opponent":
			w.Opponent = v
		case "minutes":
			w.Minutes = v
		case "home_away":
			w.HomeAway = v
		}
	}
	return w
}

// Leagues ─────────────────────────────────────────────────────────────────────

// normalizeLeagueName maps an API league name to the canonical name used
// throughout Prediplay. Returns "" if the league is not one of the 5 supported.
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

func (h *Handler) GetLeagues(w http.ResponseWriter, r *http.Request) {
	all, err := h.bzzoiro.GetLeagues()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	// Return only the 5 supported leagues with normalized names.
	filtered := make([]models.League, 0, 5)
	for _, l := range all {
		if canonical := normalizeLeagueName(l.Name); canonical != "" {
			l.Name = canonical
			filtered = append(filtered, l)
		}
	}
	writeJSON(w, http.StatusOK, filtered)
}

// Teams ───────────────────────────────────────────────────────────────────────

func (h *Handler) GetTeams(w http.ResponseWriter, r *http.Request) {
	country := r.URL.Query().Get("country")
	teams, err := h.bzzoiro.GetTeams(country)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, teams)
}

// Events ──────────────────────────────────────────────────────────────────────

func (h *Handler) GetEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	leagueParam := q.Get("league")

	if leagueParam != "" {
		// Specific league requested — pass directly to the upstream API.
		events, err := h.bzzoiro.GetEvents(
			q.Get("date_from"), q.Get("date_to"), leagueParam, q.Get("status"),
		)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, events)
		return
	}

	// No league filter: fetch the 5 supported leagues and query each in parallel.
	allLeagues, err := h.bzzoiro.GetLeagues()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	var supportedIDs []uint
	for _, l := range allLeagues {
		if normalizeLeagueName(l.Name) != "" {
			supportedIDs = append(supportedIDs, l.ID)
		}
	}

	dateFrom := q.Get("date_from")
	dateTo := q.Get("date_to")
	status := q.Get("status")

	var mu sync.Mutex
	var wg sync.WaitGroup
	var combined []models.Event

	for _, id := range supportedIDs {
		wg.Add(1)
		go func(leagueID uint) {
			defer wg.Done()
			events, err := h.bzzoiro.GetEvents(
				dateFrom, dateTo, fmt.Sprintf("%d", leagueID), status,
			)
			if err != nil {
				return
			}
			mu.Lock()
			combined = append(combined, events...)
			mu.Unlock()
		}(id)
	}
	wg.Wait()

	sort.Slice(combined, func(i, j int) bool {
		return combined[i].Date.Before(combined[j].Date)
	})
	writeJSON(w, http.StatusOK, combined)
}

// Live ────────────────────────────────────────────────────────────────────────

func (h *Handler) GetLive(w http.ResponseWriter, r *http.Request) {
	events, err := h.bzzoiro.GetLive()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, events)
}

// Predictions (Bzzoiro ML) ────────────────────────────────────────────────────

func (h *Handler) GetPredictions(w http.ResponseWriter, r *http.Request) {
	upcoming := r.URL.Query().Get("upcoming") == "true"
	preds, err := h.bzzoiro.GetPredictions(upcoming)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preds)
}

// Players (local DB) ──────────────────────────────────────────────────────────

func (h *Handler) GetPlayers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	players, err := h.prediction.GetAllPlayers(q.Get("league"), q.Get("position"), q.Get("team"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, players)
}

// Player prediction ───────────────────────────────────────────────────────────

func (h *Handler) GetPlayerPrediction(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid player id")
		return
	}
	pred, err := h.prediction.GetPlayerPrediction(uint(id), parseWeights(r))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pred)
}

// Top predictions ─────────────────────────────────────────────────────────────

func (h *Handler) GetTopPredictions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	gemFilter := "non-gems"
	if q.Get("hidden_gem") == "true" {
		gemFilter = "gems"
	}
	timeFilter := q.Get("time_filter")
	if timeFilter == "" {
		timeFilter = "recent"
	}
	preds, err := h.prediction.GetTopPredictions(
		q.Get("league"), q.Get("position"), gemFilter, timeFilter,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preds)
}

// Red flags ───────────────────────────────────────────────────────────────────

func (h *Handler) GetRedFlags(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	timeFilter := q.Get("time_filter")
	if timeFilter == "" {
		timeFilter = "recent"
	}
	result, err := h.prediction.GetRedFlags(q.Get("league"), q.Get("position"), timeFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Dashboard ───────────────────────────────────────────────────────────────────

func (h *Handler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	timeFilter := r.URL.Query().Get("time_filter")
	if timeFilter == "" {
		timeFilter = "recent"
	}
	result, err := h.prediction.GetDashboard(timeFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Benchwarmers ─────────────────────────────────────────────────────────────────

func (h *Handler) GetBenchwarmers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	timeFilter := q.Get("time_filter")
	if timeFilter == "" {
		timeFilter = "recent"
	}
	result, err := h.prediction.GetBenchwarmers(q.Get("league"), q.Get("position"), timeFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Synergy ─────────────────────────────────────────────────────────────────────

func (h *Handler) GetSynergy(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("players")
	if raw == "" {
		writeError(w, http.StatusBadRequest, "players query param required (e.g. players=1,2,3)")
		return
	}
	var ids []uint
	for _, s := range strings.Split(raw, ",") {
		id, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
		if err == nil {
			ids = append(ids, uint(id))
		}
	}
	result, err := h.prediction.GetSynergy(ids, parseWeights(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Momentum ────────────────────────────────────────────────────────────────────

func (h *Handler) GetMomentum(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("player")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "player query param required")
		return
	}
	data, err := h.prediction.GetMomentum(uint(id))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// Player photo proxy ───────────────────────────────────────────────────────────

func (h *Handler) GetPlayerPhoto(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	player, err := h.prediction.GetPlayer(uint(id))
	if err != nil || player.ApiID == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	err = h.bzzoiro.ProxyPlayerPhoto(w, func(ct string) {
		if ct != "" {
			w.Header().Set("Content-Type", ct)
		}
	}, player.ApiID)
	if err != nil {
		http.NotFound(w, r)
	}
}

// Player stats ─────────────────────────────────────────────────────────────────

func (h *Handler) GetPlayerStats(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid player id")
		return
	}
	stats, err := h.bzzoiro.GetPlayerStatsRecent(uint(id))
	if err != nil {
		// External API unavailable — return empty array so player detail still renders
		writeJSON(w, http.StatusOK, []models.PlayerStat{})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
