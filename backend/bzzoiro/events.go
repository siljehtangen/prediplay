package bzzoiro

import (
	"log"
	"prediplay/backend/models"
	"time"
)

func (c *Client) GetEvents(dateFrom, dateTo, league, status string) ([]models.Event, error) {
	params := map[string]string{}
	if dateFrom != "" {
		params["date_from"] = dateFrom
	}
	if dateTo != "" {
		params["date_to"] = dateTo
	}
	if league != "" {
		params["league"] = league
	}
	if status != "" {
		params["status"] = status
	}
	raw, err := fetchAll[rawEvent](c, "/api/events/", params)
	if err != nil {
		return nil, err
	}
	return mapEvents(raw), nil
}

func (c *Client) GetLive() ([]models.Event, error) {
	raw, err := fetchAll[rawEvent](c, "/api/live/", nil)
	if err != nil {
		return nil, err
	}
	return mapEvents(raw), nil
}

func mapEvents(raw []rawEvent) []models.Event {
	out := make([]models.Event, len(raw))
	for i, r := range raw {
		out[i] = mapEvent(r)
	}
	return out
}

func mapEvent(r rawEvent) models.Event {
	t, err := time.Parse(time.RFC3339, r.Date)
	if err != nil {
		// Some events use date-only format; try that before giving up.
		if t, err = time.Parse("2006-01-02", r.Date); err != nil {
			log.Printf("[bzzoiro] unparseable event date %q: %v", r.Date, err)
		}
	}
	return models.Event{
		ID:         r.ID,
		LeagueID:   r.League,
		HomeTeamID: r.HomeTeam.ID,
		AwayTeamID: r.AwayTeam.ID,
		HomeTeam:   models.Team{ID: r.HomeTeam.ID, Name: r.HomeTeam.Name},
		AwayTeam:   models.Team{ID: r.AwayTeam.ID, Name: r.AwayTeam.Name},
		Date:       t,
		Status:     r.Status,
	}
}
