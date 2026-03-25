package bzzoiro

import (
	"fmt"
	"io"
	"net/http"
	"prediplay/backend/models"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

type Client struct {
	http    *resty.Client
	baseURL string
	token   string
}

func New(baseURL, token string) *Client {
	r := resty.New().
		SetHeader("Authorization", "Token "+token).
		SetTimeout(15 * time.Second)
	return &Client{http: r, baseURL: baseURL, token: token}
}

// ProxyPlayerPhoto fetches the player photo from the bzzoiro image API and
// writes it directly to w. Returns an error if the image cannot be fetched.
func (c *Client) ProxyPlayerPhoto(w io.Writer, headerSetter func(string), apiID uint) error {
	url := fmt.Sprintf("%s/img/player/%d/?token=%s", c.baseURL, apiID, c.token)
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("photo not found: status %d", resp.StatusCode)
	}
	headerSetter(resp.Header.Get("Content-Type"))
	_, err = io.Copy(w, resp.Body)
	return err
}

// --- Raw API response types ---

type paginated[T any] struct {
	Count   int `json:"count"`
	Results []T `json:"results"`
}

type rawLeague struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	Country string `json:"country"`
	Active  bool   `json:"active"`
}

type rawTeam struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	Country string `json:"country"`
	League  uint   `json:"league"`
}

type rawTeamRef struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// rawEvent is used for /api/events/ and /api/live/ — home_team/away_team are objects
type rawEvent struct {
	ID       uint       `json:"id"`
	League   uint       `json:"league"`
	HomeTeam rawTeamRef `json:"home_team"`
	AwayTeam rawTeamRef `json:"away_team"`
	Date     string     `json:"date"`
	Status   string     `json:"status"`
}

// rawPredEvent is used inside /api/predictions/ — home_team/away_team are plain strings
type rawPredEvent struct {
	HomeTeam string `json:"home_team"`
	AwayTeam string `json:"away_team"`
}

// rawStatEvent is used inside /api/player-stats/ — uses event_date and string team names
type rawStatEvent struct {
	ID        uint   `json:"id"`
	APIID     uint   `json:"api_id"`
	HomeTeam  string `json:"home_team"`
	AwayTeam  string `json:"away_team"`
	EventDate string `json:"event_date"`
	HomeScore int    `json:"home_score"`
	AwayScore int    `json:"away_score"`
}

type rawPrediction struct {
	ID              uint         `json:"id"`
	Event           rawPredEvent `json:"event"`
	ProbHomeWin     float64      `json:"prob_home_win"`
	ProbDraw        float64      `json:"prob_draw"`
	ProbAwayWin     float64      `json:"prob_away_win"`
	PredictedResult string       `json:"predicted_result"`
	ProbOver25      float64      `json:"prob_over_25"`
	ProbBttsYes     float64      `json:"prob_btts_yes"`
	Confidence      float64      `json:"confidence"`
	ModelVersion    string       `json:"model_version"`
}

type rawPlayer struct {
	ID           uint       `json:"id"`
	APIID        uint       `json:"api_id"`
	Name         string     `json:"name"`
	ShortName    string     `json:"short_name"`
	Position     string     `json:"position"`
	JerseyNumber uint       `json:"jersey_number"`
	Height       uint       `json:"height"`
	DateOfBirth  string     `json:"date_of_birth"`
	Nationality  string     `json:"nationality"`
	MarketValue  uint       `json:"market_value"`
	CurrentTeam  rawTeamRef `json:"current_team"`
}

type rawPlayerStat struct {
	Event           rawStatEvent `json:"event"`
	MinutesPlayed   uint         `json:"minutes_played"`
	Rating          float64      `json:"rating"`
	Goals           uint         `json:"goals"`
	GoalAssist      uint         `json:"goal_assist"`
	ExpectedGoals   float64      `json:"expected_goals"`
	ExpectedAssists float64      `json:"expected_assists"`
	TotalShots      uint         `json:"total_shots"`
	ShotsOnTarget   uint         `json:"shots_on_target"`
	TotalPass       uint         `json:"total_pass"`
	AccuratePass    uint         `json:"accurate_pass"`
	KeyPass         uint         `json:"key_pass"`
	Touches         uint         `json:"touches"`
	DuelWon         uint         `json:"duel_won"`
	DuelLost        uint         `json:"duel_lost"`
	TotalTackle     uint         `json:"total_tackle"`
	WonTackle       uint         `json:"won_tackle"`
	YellowCard      uint         `json:"yellow_card"`
	RedCard         uint         `json:"red_card"`
	Saves           uint         `json:"saves"`
	GoalsConceded   uint         `json:"goals_conceded"`
}

// --- Fetch helpers ---

func fetchAll[Raw any](c *Client, path string, params map[string]string) ([]Raw, error) {
	var all []Raw
	page := 1
	for {
		var resp paginated[Raw]
		url := c.baseURL + path
		req := c.http.R().SetResult(&resp)
		for k, v := range params {
			req = req.SetQueryParam(k, v)
		}
		req = req.SetQueryParam("page", fmt.Sprintf("%d", page))

		r, err := req.Get(url)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}
		if r.IsError() {
			return nil, fmt.Errorf("API error %d: %s", r.StatusCode(), r.String())
		}
		all = append(all, resp.Results...)
		if len(all) >= resp.Count || len(resp.Results) == 0 {
			break
		}
		page++
	}
	return all, nil
}

// --- Public methods ---

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

func (c *Client) GetPredictions(upcoming bool) ([]models.Prediction, error) {
	params := map[string]string{}
	if upcoming {
		params["upcoming"] = "true"
	}
	raw, err := fetchAll[rawPrediction](c, "/api/predictions/", params)
	if err != nil {
		return nil, err
	}
	out := make([]models.Prediction, len(raw))
	for i, r := range raw {
		out[i] = models.Prediction{
			ID:              r.ID,
			HomeTeam:        r.Event.HomeTeam,
			AwayTeam:        r.Event.AwayTeam,
			ProbHomeWin:     r.ProbHomeWin,
			ProbDraw:        r.ProbDraw,
			ProbAwayWin:     r.ProbAwayWin,
			PredictedResult: r.PredictedResult,
			ProbOver25:      r.ProbOver25,
			ProbBttsYes:     r.ProbBttsYes,
			Confidence:      r.Confidence,
			ModelVersion:    r.ModelVersion,
		}
	}
	return out, nil
}

func mapEvents(raw []rawEvent) []models.Event {
	out := make([]models.Event, len(raw))
	for i, r := range raw {
		out[i] = mapEvent(r)
	}
	return out
}

func mapEvent(r rawEvent) models.Event {
	t, _ := time.Parse(time.RFC3339, r.Date)
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

// GetPlayerStats fetches all historical stats for a player (full season).
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

// GetPlayerStatsRecent fetches only the first page of stats (most recent games).
// Used during SyncPlayers to populate the DB cache quickly.
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

// GetPlayersFirstPage fetches only the first page of players (no pagination).
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
