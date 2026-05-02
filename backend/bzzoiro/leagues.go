package bzzoiro

import (
	"fmt"
	"prediplay/backend/models"
)

// GetLeagues returns all leagues from the bzzoiro API.
func (c *Client) GetLeagues() ([]models.League, error) {
	raw, err := fetchAll[rawLeague](c, "/api/leagues/", nil)
	if err != nil {
		return nil, err
	}
	out := make([]models.League, len(raw))
	for i, r := range raw {
		out[i] = models.League{ID: r.ID, Name: r.Name, Country: r.Country, Active: r.Active}
	}
	return out, nil
}

// GetTeams returns teams filtered by country and optionally by league ID.
func (c *Client) GetTeams(country string, leagueID ...uint) ([]models.Team, error) {
	params := map[string]string{}
	if country != "" {
		params["country"] = country
	}
	if len(leagueID) > 0 && leagueID[0] != 0 {
		params["league"] = fmt.Sprintf("%d", leagueID[0])
	}
	raw, err := fetchAll[rawTeam](c, "/api/teams/", params)
	if err != nil {
		return nil, err
	}
	out := make([]models.Team, len(raw))
	for i, r := range raw {
		out[i] = models.Team{ID: r.ID, Name: r.Name, Country: r.Country, LeagueID: r.League}
	}
	return out, nil
}
