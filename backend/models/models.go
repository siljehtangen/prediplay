package models

import "time"

// League represents a football league stored in the database.
type League struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	Name    string `json:"name"`
	Country string `json:"country"`
	Active  bool   `json:"active"`
}

// Team represents a football team stored in the database.
type Team struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Name     string `json:"name"`
	Country  string `json:"country"`
	LeagueID uint   `json:"league_id"`
}

// Player represents a football player with full-season and recent-form statistics.
// Position is one of GK, DEF, MID, FWD.
type Player struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	APIID        uint   `gorm:"column:api_id" json:"api_id"`
	Name         string `json:"name"`
	ShortName    string `json:"short_name"`
	TeamID       uint   `json:"team_id"`
	TeamName     string `json:"team_name"`
	League       string `gorm:"index" json:"league"`
	Position     string `gorm:"index" json:"position"` // GK, DEF, MID, FWD
	JerseyNumber uint   `json:"jersey_number"`
	Height       uint   `json:"height"`
	DateOfBirth  string `json:"date_of_birth"`
	Nationality  string `json:"nationality"`
	MarketValue  uint   `json:"market_value"`

	GamesPlayed    int     `json:"games_played"`
	MinutesPlayed  int     `gorm:"index" json:"minutes_played"`
	Goals          int     `json:"goals"`
	Assists        int     `json:"assists"`
	XG             float64 `json:"xG"`
	XA             float64 `json:"xA"`
	TotalShots     int     `json:"total_shots"`
	ShotsOnTarget  int     `json:"shots_on_target"`
	KeyPasses      int     `json:"key_passes"`
	TotalPasses    int     `json:"total_passes"`
	AccuratePasses int     `json:"accurate_passes"`
	DuelsWon       int     `json:"duels_won"`
	DuelsTotal     int     `json:"duels_total"`
	TacklesWon     int     `json:"tackles_won"`
	TacklesTotal   int     `json:"tackles_total"`
	YellowCards    int     `json:"yellow_cards"`
	RedCards       int     `json:"red_cards"`
	Saves          int     `json:"saves"`          // GK only
	GoalsConceded  int     `json:"goals_conceded"` // GK only
	FormScore      float64 `json:"form_score"`     // average match rating (1–10)

	RecentGamesPlayed    int     `json:"recent_games_played"`
	RecentMinutes        int     `json:"recent_minutes"`
	RecentGoals          int     `json:"recent_goals"`
	RecentAssists        int     `json:"recent_assists"`
	RecentXG             float64 `json:"recent_xg"`
	RecentXA             float64 `json:"recent_xa"`
	RecentTotalShots     int     `json:"recent_total_shots"`
	RecentShotsOnTarget  int     `json:"recent_shots_on_target"`
	RecentKeyPasses      int     `json:"recent_key_passes"`
	RecentTotalPasses    int     `json:"recent_total_passes"`
	RecentAccuratePasses int     `json:"recent_accurate_passes"`
	RecentDuelsWon       int     `json:"recent_duels_won"`
	RecentDuelsTotal     int     `json:"recent_duels_total"`
	RecentTacklesWon     int     `json:"recent_tackles_won"`
	RecentTacklesTotal   int     `json:"recent_tackles_total"`
	RecentYellowCards    int     `json:"recent_yellow_cards"`
	RecentRedCards       int     `json:"recent_red_cards"`
	RecentSaves          int     `json:"recent_saves"`
	RecentGoalsConceded  int     `json:"recent_goals_conceded"`
	RecentFormScore      float64 `json:"recent_form_score"`

	NextOpponent  string  `json:"next_opponent"`
	OpponentScore float64 `json:"opponent_score"` // 0–10: historical performance vs that opponent
	IsHome        bool    `json:"is_home"`
	LastMatchDate string  `json:"last_match_date"` // YYYY-MM-DD of the most recent played game
}

// Event represents a scheduled or completed football match.
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

// Prediction maps directly to the /api/predictions/ response.
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

// StatEvent is the event embedded inside player-stats responses.
type StatEvent struct {
	ID        uint   `json:"id"`
	HomeTeam  string `json:"home_team"`
	AwayTeam  string `json:"away_team"`
	EventDate string `json:"event_date"`
	HomeScore int    `json:"home_score"`
	AwayScore int    `json:"away_score"`
}

// PlayerStat is a single per-match stat record from /api/player-stats/.
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

// PlayerPrediction is computed on the fly (not stored in DB).
// Contribution fields carry the raw component scores (0–10) for the frontend.
type PlayerPrediction struct {
	Player                Player   `json:"player"`
	PredictedScore        float64  `json:"predicted_score"`
	RiskLevel             string   `json:"risk_level"` // low, medium, high
	HiddenGem             bool     `json:"hidden_gem"`
	HiddenGemReasons      []string `json:"hidden_gem_reasons,omitempty"`
	FormContribution      float64  `json:"form_contribution"`
	ThreatContribution    float64  `json:"threat_contribution"`
	OpponentDifficulty    float64  `json:"opponent_difficulty"`
	MinutesLikelihood     float64  `json:"minutes_likelihood"`
	DefensiveContribution float64  `json:"defensive_contribution"`
	NextEvent             *Event   `json:"next_event,omitempty"`
}

// RedFlagPlayer pairs a player with their computed decline signals.
type RedFlagPlayer struct {
	Player       Player   `json:"player"`
	RedFlagScore float64  `json:"red_flag_score"` // 0–10, higher = more alarming
	FormDecline  float64  `json:"form_decline"`
	OutputDrop   float64  `json:"output_drop"`
	Reasons      []string `json:"reasons"`
}

// BenchwarmerPlayer pairs a player with their consistency score and descriptive label.
type BenchwarmerPlayer struct {
	Player           Player  `json:"player"`
	ConsistencyScore float64 `json:"consistency_score"` // 0–10
	Label            string  `json:"label"`             // e.g. "Rock Solid", "Steady Option"
}

// DashboardLeague groups top predictions and red flags for a single league.
type DashboardLeague struct {
	Name       string             `json:"name"`
	TopPlayers []PlayerPrediction `json:"top_players"`
	RedFlags   []RedFlagPlayer    `json:"red_flags"`
}

// MomentumGame holds the performance data for a single match in a momentum series.
type MomentumGame struct {
	Date     string  `json:"date"`
	Opponent string  `json:"opponent"`
	Score    float64 `json:"score"`
	Goals    int     `json:"goals"`
	Assists  int     `json:"assists"`
	Minutes  int     `json:"minutes"`
}

// MomentumData describes a player's performance trend over their last few games.
type MomentumData struct {
	Player Player         `json:"player"`
	Games  []MomentumGame `json:"games"`
	Trend  string         `json:"trend"` // rising, falling, stable
}

// SynergyResult holds the combined prediction score for a group of players.
type SynergyResult struct {
	Players        []Player `json:"players"`
	TotalPredicted float64  `json:"total_predicted"`
	SynergyBonus   float64  `json:"synergy_bonus"`
	SynergyScore   float64  `json:"synergy_score"`
}
