package handlers

import (
	"net/http"
	"strconv"
	"strings"
)

func (h *Handler) GetPlayers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	players, err := h.prediction.GetAllPlayers(q.Get("league"), q.Get("position"), q.Get("team"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, players)
}

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

func (h *Handler) GetRedFlags(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	result, err := h.prediction.GetRedFlags(q.Get("league"), q.Get("position"), timeFilterParam(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	result, err := h.prediction.GetDashboard(timeFilterParam(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetBenchwarmers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	result, err := h.prediction.GetBenchwarmers(q.Get("league"), q.Get("position"), timeFilterParam(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

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
	result, err := h.prediction.GetSynergy(ids)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

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
