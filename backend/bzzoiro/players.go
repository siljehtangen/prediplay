package bzzoiro

import (
	"fmt"
	"prediplay/backend/models"
	"strings"
)

func (c *Client) GetPlayers(position, nationality, team string) ([]models.Player, error) {
	params := map[string]string{}
	if position != "" {
		params["position"] = position
	}
	if nationality != "" {
		params["nationality"] = nationality
	}
	if team != "" {
		params["team"] = team
	}
	raw, err := fetchAll[rawPlayer](c, "/api/players/", params)
	if err != nil {
		return nil, err
	}
	out := make([]models.Player, len(raw))
	for i, r := range raw {
		out[i] = mapRawPlayer(r)
	}
	return out, nil
}

func (c *Client) GetPlayersFirstPage(position, teamID string) ([]models.Player, error) {
	var resp paginated[rawPlayer]
	url := c.baseURL + "/api/players/"
	req := c.http.R().SetResult(&resp)
	if position != "" {
		req = req.SetQueryParam("position", position)
	}
	if teamID != "" {
		req = req.SetQueryParam("team", teamID)
	}
	r, err := req.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if r.IsError() {
		return nil, fmt.Errorf("API error %d: %s", r.StatusCode(), r.String())
	}
	out := make([]models.Player, len(resp.Results))
	for i, rp := range resp.Results {
		out[i] = mapRawPlayer(rp)
	}
	return out, nil
}

func normalizePosition(p string) string {
	switch strings.ToUpper(strings.TrimSpace(p)) {
	case "G", "GK", "GOALKEEPER":
		return "GK"
	case "D", "DEF", "DEFENDER", "CB", "LB", "RB", "LWB", "RWB":
		return "DEF"
	case "M", "MID", "MIDFIELDER", "CM", "CAM", "CDM", "LM", "RM", "DM", "AM":
		return "MID"
	case "F", "FWD", "FORWARD", "ST", "CF", "LW", "RW", "SS", "A", "ATT", "ATTACKER":
		return "FWD"
	}
	return p
}

func mapRawPlayer(r rawPlayer) models.Player {
	return models.Player{
		ID:           r.ID,
		ApiID:        r.APIID,
		Name:         r.Name,
		ShortName:    r.ShortName,
		TeamID:       r.CurrentTeam.ID,
		TeamName:     r.CurrentTeam.Name,
		Position:     normalizePosition(r.Position),
		JerseyNumber: r.JerseyNumber,
		Height:       r.Height,
		DateOfBirth:  r.DateOfBirth,
		Nationality:  r.Nationality,
		MarketValue:  r.MarketValue,
	}
}

func (c *Client) GetPlayerStats(playerID uint) ([]models.PlayerStat, error) {
	params := map[string]string{"player": fmt.Sprintf("%d", playerID)}
	raw, err := fetchAll[rawPlayerStat](c, "/api/player-stats/", params)
	if err != nil {
		return nil, err
	}
	out := make([]models.PlayerStat, len(raw))
	for i, r := range raw {
		out[i] = mapRawStat(r)
	}
	return out, nil
}

// GetPlayerStatsSince fetches stats for a player from dateFrom onwards (YYYY-MM-DD).
// Passes date_from to the API if supported; always filters client-side as a fallback.
func (c *Client) GetPlayerStatsSince(playerID uint, dateFrom string) ([]models.PlayerStat, error) {
	params := map[string]string{
		"player":    fmt.Sprintf("%d", playerID),
		"date_from": dateFrom,
	}
	raw, err := fetchAll[rawPlayerStat](c, "/api/player-stats/", params)
	if err != nil {
		return nil, err
	}
	out := make([]models.PlayerStat, 0, len(raw))
	for _, r := range raw {
		// Client-side fallback: keep only stats on or after dateFrom
		eventDate := r.Event.EventDate
		if len(eventDate) >= 10 {
			eventDate = eventDate[:10] // trim to YYYY-MM-DD
		}
		if eventDate >= dateFrom {
			out = append(out, mapRawStat(r))
		}
	}
	return out, nil
}

func (c *Client) GetPlayerStatsRecent(playerID uint) ([]models.PlayerStat, error) {
	var resp paginated[rawPlayerStat]
	url := c.baseURL + "/api/player-stats/"
	r, err := c.http.R().SetResult(&resp).
		SetQueryParam("player", fmt.Sprintf("%d", playerID)).
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if r.IsError() {
		return nil, fmt.Errorf("API error %d: %s", r.StatusCode(), r.String())
	}
	out := make([]models.PlayerStat, len(resp.Results))
	for i, rs := range resp.Results {
		out[i] = mapRawStat(rs)
	}
	return out, nil
}

func mapRawStat(r rawPlayerStat) models.PlayerStat {
	return models.PlayerStat{
		Event: models.StatEvent{
			ID:        r.Event.ID,
			HomeTeam:  r.Event.HomeTeam,
			AwayTeam:  r.Event.AwayTeam,
			EventDate: r.Event.EventDate,
			HomeScore: r.Event.HomeScore,
			AwayScore: r.Event.AwayScore,
		},
		MinutesPlayed:   r.MinutesPlayed,
		Rating:          r.Rating,
		Goals:           r.Goals,
		GoalAssist:      r.GoalAssist,
		ExpectedGoals:   r.ExpectedGoals,
		ExpectedAssists: r.ExpectedAssists,
		TotalShots:      r.TotalShots,
		ShotsOnTarget:   r.ShotsOnTarget,
		TotalPass:       r.TotalPass,
		AccuratePass:    r.AccuratePass,
		KeyPass:         r.KeyPass,
		Touches:         r.Touches,
		DuelWon:         r.DuelWon,
		DuelLost:        r.DuelLost,
		TotalTackle:     r.TotalTackle,
		WonTackle:       r.WonTackle,
		YellowCard:      r.YellowCard,
		RedCard:         r.RedCard,
		Saves:           r.Saves,
		GoalsConceded:   r.GoalsConceded,
	}
}
