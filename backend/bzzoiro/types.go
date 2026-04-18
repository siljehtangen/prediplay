package bzzoiro

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
