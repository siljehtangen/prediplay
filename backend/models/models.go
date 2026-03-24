package models

import "time"

type League struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	Name    string `json:"name"`
	Country string `json:"country"`
	Active  bool   `json:"active"`
}

type Team struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Name     string `json:"name"`
	Country  string `json:"country"`
	LeagueID uint   `json:"league_id"`
}

type Player struct {
	ID            uint    `gorm:"primaryKey" json:"id"`
	ApiID         uint    `json:"api_id"`
	Name          string  `json:"name"`
	ShortName     string  `json:"short_name"`
	TeamID        uint    `json:"team_id"`
	TeamName      string  `json:"team_name"`
	League        string  `json:"league"`
	Position      string  `json:"position"` // GK, DEF, MID, FWD
	JerseyNumber  uint    `json:"jersey_number"`
	Height        uint    `json:"height"`
	DateOfBirth   string  `json:"date_of_birth"`
	Nationality   string  `json:"nationality"`
	MarketValue   uint    `json:"market_value"`
	MinutesPlayed int     `json:"minutes_played"`
	Goals         int     `json:"goals"`
	Assists       int     `json:"assists"`
	XG            float64 `json:"xG"`
	XA            float64 `json:"xA"`
	FormScore     float64 `json:"form_score"` // 0-10
}

type Event struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	LeagueID   uint      `json:"league_id"`
	HomeTeamID uint      `json:"home_team_id"`
	AwayTeamID uint      `json:"away_team_id"`
	HomeTeam   Team      `gorm:"-" json:"home_team"`
	AwayTeam   Team      `gorm:"-" json:"away_team"`
	Date       time.Time `json:"date"`
	Status     string    `json:"status"`
}

// Prediction maps directly to the /api/predictions/ response
type Prediction struct {
	ID              uint    `json:"id"`
	HomeTeam        string  `json:"home_team"`
	AwayTeam        string  `json:"away_team"`
	ProbHomeWin     float64 `json:"prob_home_win"`
	ProbDraw        float64 `json:"prob_draw"`
	ProbAwayWin     float64 `json:"prob_away_win"`
	PredictedResult string  `json:"predicted_result"`
	ProbOver25      float64 `json:"prob_over_25"`
	ProbBttsYes     float64 `json:"prob_btts_yes"`
	Confidence      float64 `json:"confidence"`
	ModelVersion    string  `json:"model_version"`
}

// StatEvent is the event embedded inside player-stats responses
type StatEvent struct {
	ID        uint   `json:"id"`
	HomeTeam  string `json:"home_team"`
	AwayTeam  string `json:"away_team"`
	EventDate string `json:"event_date"`
	HomeScore int    `json:"home_score"`
	AwayScore int    `json:"away_score"`
}

// PlayerStat is a single per-match stat record from /api/player-stats/
type PlayerStat struct {
	Event           StatEvent `json:"event"`
	MinutesPlayed   uint      `json:"minutes_played"`
	Rating          float64   `json:"rating"`
	Goals           uint      `json:"goals"`
	GoalAssist      uint      `json:"goal_assist"`
	ExpectedGoals   float64   `json:"expected_goals"`
	ExpectedAssists float64   `json:"expected_assists"`
	TotalShots      uint      `json:"total_shots"`
	ShotsOnTarget   uint      `json:"shots_on_target"`
	TotalPass       uint      `json:"total_pass"`
	AccuratePass    uint      `json:"accurate_pass"`
	KeyPass         uint      `json:"key_pass"`
	Touches         uint      `json:"touches"`
	DuelWon         uint      `json:"duel_won"`
	DuelLost        uint      `json:"duel_lost"`
	TotalTackle     uint      `json:"total_tackle"`
	WonTackle       uint      `json:"won_tackle"`
	YellowCard      uint      `json:"yellow_card"`
	RedCard         uint      `json:"red_card"`
	Saves           uint      `json:"saves"`
	GoalsConceded   uint      `json:"goals_conceded"`
}

// PlayerPrediction is not stored in DB, computed on the fly
type PlayerPrediction struct {
	Player             Player  `json:"player"`
	PredictedScore     float64 `json:"predicted_score"`
	RiskLevel          string  `json:"risk_level"` // low, medium, high
	HiddenGem          bool    `json:"hidden_gem"`
	FormContribution   float64 `json:"form_contribution"`
	ThreatContribution float64 `json:"threat_contribution"`
	OpponentDifficulty float64 `json:"opponent_difficulty"`
	MinutesLikelihood  float64 `json:"minutes_likelihood"`
	HomeAwayFactor     float64 `json:"home_away_factor"`
	NextEvent          *Event  `json:"next_event,omitempty"`
}

// RedFlagPlayer represents a player showing worrying form/output trends
type RedFlagPlayer struct {
	Player       Player   `json:"player"`
	RedFlagScore float64  `json:"red_flag_score"` // 0-10, higher = more alarming
	FormDecline  float64  `json:"form_decline"`
	OutputDrop   float64  `json:"output_drop"`
	Reasons      []string `json:"reasons"`
}

// BenchwarmerPlayer represents a consistent, reliable but non-elite player
type BenchwarmerPlayer struct {
	Player           Player  `json:"player"`
	ConsistencyScore float64 `json:"consistency_score"` // 0-10
	Label            string  `json:"label"`             // "Rock Solid", "Steady Option", "Rotation Pick"
}

type MomentumGame struct {
	Date     string  `json:"date"`
	Opponent string  `json:"opponent"`
	Score    float64 `json:"score"`
	Goals    int     `json:"goals"`
	Assists  int     `json:"assists"`
	Minutes  int     `json:"minutes"`
}

type MomentumData struct {
	Player Player         `json:"player"`
	Games  []MomentumGame `json:"games"`
	Trend  string         `json:"trend"` // rising, falling, stable
}

type SynergyResult struct {
	Players        []Player `json:"players"`
	TotalPredicted float64  `json:"total_predicted"`
	SynergyBonus   float64  `json:"synergy_bonus"`
	SynergyScore   float64  `json:"synergy_score"`
}

type PredictionWeights struct {
	Form     float64
	Threat   float64
	Opponent float64
	Minutes  float64
	HomeAway float64
}

func DefaultWeights() PredictionWeights {
	return PredictionWeights{
		Form:     0.35,
		Threat:   0.25,
		Opponent: 0.15,
		Minutes:  0.15,
		HomeAway: 0.10,
	}
}
