package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"sync"

	"prediplay/backend/models"
)

func (h *Handler) GetLeagues(w http.ResponseWriter, r *http.Request) {
	all, err := h.bzzoiro.GetLeagues()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	filtered := make([]models.League, 0, 5)
	for _, l := range all {
		if canonical := normalizeLeagueName(l.Name); canonical != "" {
			l.Name = canonical
			filtered = append(filtered, l)
		}
	}
	writeJSON(w, http.StatusOK, filtered)
}

func (h *Handler) GetTeams(w http.ResponseWriter, r *http.Request) {
	country := r.URL.Query().Get("country")
	teams, err := h.bzzoiro.GetTeams(country)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, teams)
}

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

func (h *Handler) GetLive(w http.ResponseWriter, r *http.Request) {
	events, err := h.bzzoiro.GetLive()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (h *Handler) GetPredictions(w http.ResponseWriter, r *http.Request) {
	upcoming := r.URL.Query().Get("upcoming") == "true"
	preds, err := h.bzzoiro.GetPredictions(upcoming)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preds)
}

func (h *Handler) GetPlayerPhoto(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePlayerID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	player, err := h.prediction.GetPlayer(id)
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

func (h *Handler) GetPlayerStats(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePlayerID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid player id")
		return
	}
	stats, err := h.bzzoiro.GetPlayerStatsRecent(id)
	if err != nil {
		// External API unavailable — return empty array so player detail still renders
		writeJSON(w, http.StatusOK, []models.PlayerStat{})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
