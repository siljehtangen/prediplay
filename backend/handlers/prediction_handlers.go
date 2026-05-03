package handlers

import (
	"net/http"
	"strconv"
	"strings"
)

// GetPlayers returns all players filtered by league, position, and team name.
func (h *Handler) GetPlayers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	players, err := h.prediction.GetAllPlayers(q.Get("league"), q.Get("position"), q.Get("team"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, players)
}

// GetPlayerPrediction returns the computed prediction for a single player by ID.
func (h *Handler) GetPlayerPrediction(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePlayerID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid player id")
		return
	}
	pred, err := h.prediction.GetPlayerPrediction(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pred)
}

// GetTopPredictions returns the top-ranked predictions filtered by league, position,
// and hidden-gem status (query param: hidden_gem=true).
func (h *Handler) GetTopPredictions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	gemFilter := "non-gems"
	if q.Get("hidden_gem") == "true" {
		gemFilter = "gems"
	}
	preds, err := h.prediction.GetTopPredictions(
		q.Get("league"), q.Get("position"), gemFilter, timeFilterParam(r),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preds)
}

// GetRedFlags returns players with declining performance signals, filtered by league and position.
func (h *Handler) GetRedFlags(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	result, err := h.prediction.GetRedFlags(q.Get("league"), q.Get("position"), timeFilterParam(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetDashboard returns a per-league summary of top predictions and red flags.
func (h *Handler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	result, err := h.prediction.GetDashboard(timeFilterParam(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetBenchwarmers returns consistently reliable players filtered by league and position.
func (h *Handler) GetBenchwarmers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	result, err := h.prediction.GetBenchwarmers(q.Get("league"), q.Get("position"), timeFilterParam(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetSynergy returns the combined prediction score for a comma-separated list of player IDs
// (query param: players=1,2,3).
func (h *Handler) GetSynergy(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("players")
	if raw == "" {
		writeError(w, http.StatusBadRequest, "players query param required (e.g. players=1,2,3)")
		return
	}
	const maxSynergyPlayers = 11
	var ids []uint
	for _, s := range strings.Split(raw, ",") {
		if len(ids) == maxSynergyPlayers {
			break
		}
		id, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
		if err == nil {
			ids = append(ids, uint(id))
		}
	}
	if len(ids) == 0 {
		writeError(w, http.StatusBadRequest, "no valid player IDs provided")
		return
	}
	result, err := h.prediction.GetSynergy(ids)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetMomentum returns a player's performance trend over their recent games
// (query param: player=<id>).
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
