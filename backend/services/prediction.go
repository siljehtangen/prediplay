package services

import (
	"fmt"
	"math"
	"math/rand"
	"prediplay/backend/bzzoiro"
	"prediplay/backend/models"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

type PredictionService struct {
	db     *gorm.DB
	client *bzzoiro.Client
}

func NewPredictionService(db *gorm.DB, client *bzzoiro.Client) *PredictionService {
	return &PredictionService{db: db, client: client}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// aggregateStats computes aggregated stats from raw per-game records into a Player struct.
func aggregateStats(p *models.Player, stats []models.PlayerStat) {
	var totalMinutes, totalGoals, totalAssists uint
	var totalXG, totalXA, totalRating float64
	var games int
	for _, st := range stats {
		totalMinutes += st.MinutesPlayed
		totalGoals += st.Goals
		totalAssists += st.GoalAssist
		totalXG += st.ExpectedGoals
		totalXA += st.ExpectedAssists
		if st.Rating > 0 {
			totalRating += st.Rating
			games++
		}
	}
	p.MinutesPlayed = int(totalMinutes)
	p.Goals = int(totalGoals)
	p.Assists = int(totalAssists)
	p.XG = totalXG
	p.XA = totalXA
	if games > 0 {
		p.FormScore = totalRating / float64(games)
	} else if p.FormScore == 0 {
		p.FormScore = 6.5
	}
}

// enrichPlayersWithStats fetches recent stats for each player in parallel (max 5 concurrent).
// It updates each player in-place and upserts to the local DB for future use.
func (s *PredictionService) enrichPlayersWithStats(players []models.Player) []models.Player {
	sem := make(chan struct{}, 5)
	var mu sync.Mutex
	var wg sync.WaitGroup
	result := make([]models.Player, 0, len(players))

	for _, p := range players {
		wg.Add(1)
		player := p
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			stats, err := s.client.GetPlayerStatsRecent(player.ID)
			if err == nil {
				aggregateStats(&player, stats)
			}

			// Upsert into local DB so player detail / synergy work immediately
			s.db.Save(&player)

			mu.Lock()
			result = append(result, player)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return result
}

// enrichPlayersWithAllStats fetches full season stats for each player in parallel (max 5 concurrent).
// Does NOT update the DB cache so the "recent" cache remains intact.
func (s *PredictionService) enrichPlayersWithAllStats(players []models.Player) []models.Player {
	sem := make(chan struct{}, 5)
	var mu sync.Mutex
	var wg sync.WaitGroup
	result := make([]models.Player, 0, len(players))

	for _, p := range players {
		wg.Add(1)
		player := p
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			stats, err := s.client.GetPlayerStats(player.ID)
			if err == nil {
				aggregateStats(&player, stats)
			}

			mu.Lock()
			result = append(result, player)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return result
}

// targetLeagues maps country name to the official league name used throughout the app.
var targetLeagues = map[string]string{
	"England": "Premier League",
	"Spain":   "La Liga",
	"Germany": "Bundesliga",
	"Italy":   "Serie A",
	"France":  "Ligue 1",
}

// SyncPlayers runs in the background and keeps the local DB populated with fresh
// player data across the top 5 European leagues. Safe to call at every startup.
func (s *PredictionService) SyncPlayers() {
	fmt.Println("[sync] Starting player sync…")

	// Fetch leagues from the API so we can filter teams to the correct division only.
	apiLeagues, err := s.client.GetLeagues()
	if err != nil {
		fmt.Printf("[sync] Warning: could not fetch leagues for ID mapping: %v\n", err)
	}
	leagueIDByName := map[string]uint{}
	for _, l := range apiLeagues {
		leagueIDByName[l.Name] = l.ID
	}

	for country, leagueName := range targetLeagues {
		teams, err := s.client.GetTeams(country)
		if err != nil {
			fmt.Printf("[sync] Warning: teams for %s: %v\n", country, err)
			continue
		}

		// Only sync teams that belong to the target top-flight league.
		// This prevents Championship/lower-league teams from being tagged as "Premier League" etc.
		targetLeagueID := leagueIDByName[leagueName]

		for _, team := range teams {
			if targetLeagueID != 0 && team.LeagueID != targetLeagueID {
				continue
			}

			players, err := s.client.GetPlayersFirstPage("", fmt.Sprintf("%d", team.ID))
			if err != nil {
				fmt.Printf("[sync] Warning: players for team %s: %v\n", team.Name, err)
				continue
			}

			for i := range players {
				players[i].League = leagueName

				stats, err := s.client.GetPlayerStatsRecent(players[i].ID)
				if err == nil {
					aggregateStats(&players[i], stats)
				}

				if err := s.db.Save(&players[i]).Error; err != nil {
					fmt.Printf("[sync] Warning: save player %s: %v\n", players[i].Name, err)
				}
			}
		}
	}
	fmt.Println("[sync] Player sync complete")
}

// GetPlayer returns a single player by ID from the local DB.
func (s *PredictionService) GetPlayer(playerID uint) (models.Player, error) {
	var p models.Player
	return p, s.db.First(&p, playerID).Error
}

// GetAllPlayers returns players from the local DB, used by the synergy picker.
func (s *PredictionService) GetAllPlayers(league, position, team string) ([]models.Player, error) {
	query := s.db.Model(&models.Player{})
	if league != "" {
		query = query.Where("league = ?", league)
	}
	if position != "" {
		query = query.Where("position = ?", position)
	}
	if team != "" {
		query = query.Where("team_name LIKE ?", "%"+team+"%")
	}
	var players []models.Player
	if err := query.Find(&players).Error; err != nil {
		return nil, err
	}
	return players, nil
}

// GetPlayerPrediction fetches fresh stats for the player from the API, then computes prediction.
func (s *PredictionService) GetPlayerPrediction(playerID uint, w models.PredictionWeights) (*models.PlayerPrediction, error) {
	// Load base player info from DB
	var player models.Player
	if err := s.db.First(&player, playerID).Error; err != nil {
		return nil, fmt.Errorf("player not found: %w", err)
	}

	// Always refresh stats from the live API
	stats, err := s.client.GetPlayerStatsRecent(playerID)
	if err == nil && len(stats) > 0 {
		aggregateStats(&player, stats)
		s.db.Save(&player) // keep DB in sync
	}

	return s.calcPrediction(player, w), nil
}

// supportedLeagueNames returns the canonical names of all 5 supported leagues.
func supportedLeagueNames() []string {
	names := make([]string, 0, len(targetLeagues))
	for _, name := range targetLeagues {
		names = append(names, name)
	}
	return names
}

// GetTopPredictions queries players from the local DB (populated via SyncPlayers from the API),
// computes prediction scores, and returns the top 10 sorted by predicted score.
//
// Always restricts to the 5 supported leagues to prevent stale data from leaking in.
// When hiddenGemOnly=false: returns top performers that are NOT hidden gems.
// When hiddenGemOnly=true:  returns only hidden gem players.
// This ensures the two lists are always distinct with no overlap.
func (s *PredictionService) GetTopPredictions(league, position string, hiddenGemOnly bool, w models.PredictionWeights, timeFilter string) ([]models.PlayerPrediction, error) {
	// Always scope to the 5 supported leagues regardless of whether a specific one is requested.
	query := s.db.Model(&models.Player{}).Where("league IN ?", supportedLeagueNames())
	if league != "" {
		query = query.Where("league = ?", league)
	}
	if position != "" {
		query = query.Where("position = ?", position)
	}
	var players []models.Player
	if err := query.Find(&players).Error; err != nil {
		return nil, err
	}

	// For "overall", re-enrich players with full season stats (all pages).
	// For "recent" (default), use the DB-cached values from the most recent games.
	if timeFilter == "overall" {
		players = s.enrichPlayersWithAllStats(players)
	}

	preds := make([]models.PlayerPrediction, 0, len(players))
	for _, p := range players {
		pred := s.calcPrediction(p, w)
		if hiddenGemOnly && !pred.HiddenGem {
			continue // hidden gems page: skip non-gems
		}
		if !hiddenGemOnly && pred.HiddenGem {
			continue // top players page: skip hidden gems so lists don't overlap
		}
		preds = append(preds, *pred)
	}

	// Sort descending by predicted score
	for i := 1; i < len(preds); i++ {
		for j := i; j > 0 && preds[j].PredictedScore > preds[j-1].PredictedScore; j-- {
			preds[j], preds[j-1] = preds[j-1], preds[j]
		}
	}
	if len(preds) > 12 {
		preds = preds[:12]
	}
	return preds, nil
}

// calcRedFlag scores a player's alarming indicators (0-10, higher = worse).
// Returns the score, form decline sub-score, output drop sub-score, and reasons.
func calcRedFlag(p models.Player) (score, formDecline, outputDrop float64, reasons []string) {
	// Form decline: how far below "acceptable" average (7.0)?
	formDecline = math.Max(0, math.Min(10, (7.0-p.FormScore)/7.0*10))

	// Output drop: position-adjusted goals+assists per 90 vs expected baseline
	minutes90 := math.Max(1, float64(p.MinutesPlayed)/90.0)
	gA := float64(p.Goals + p.Assists)
	outputPer90 := gA / minutes90

	switch p.Position {
	case "GK":
		outputDrop = 0 // keepers aren't judged on G+A
	case "DEF":
		outputDrop = math.Max(0, math.Min(10, (0.15-outputPer90)/0.15*10))
	case "MID":
		outputDrop = math.Max(0, math.Min(10, (0.25-outputPer90)/0.25*10))
	default: // FWD
		outputDrop = math.Max(0, math.Min(10, (0.40-outputPer90)/0.40*10))
	}

	score = math.Round((formDecline*0.55+outputDrop*0.45)*100) / 100

	if formDecline >= 7 {
		reasons = append(reasons, "Form has collapsed")
	} else if formDecline >= 4 {
		reasons = append(reasons, "Below-average form")
	}
	if p.Goals+p.Assists == 0 && p.MinutesPlayed >= 900 {
		reasons = append(reasons, "No returns despite regular play")
	} else if outputDrop >= 7 && p.Position != "GK" {
		reasons = append(reasons, "Very low attacking output")
	} else if outputDrop >= 4 && p.Position != "GK" {
		reasons = append(reasons, "Below-par contributions for position")
	}
	if p.XG+p.XA < 0.5 && p.MinutesPlayed >= 450 && p.Position != "GK" {
		reasons = append(reasons, "Minimal threat (xG+xA near zero)")
	}
	return
}

// GetRedFlags returns players with alarming form/output — players to consider dropping.
func (s *PredictionService) GetRedFlags(league, position, timeFilter string) ([]models.RedFlagPlayer, error) {
	query := s.db.Model(&models.Player{}).
		Where("league IN ?", supportedLeagueNames()).
		Where("minutes_played >= ?", 270)
	if league != "" {
		query = query.Where("league = ?", league)
	}
	if position != "" {
		query = query.Where("position = ?", position)
	}
	var players []models.Player
	if err := query.Find(&players).Error; err != nil {
		return nil, err
	}

	if timeFilter == "overall" {
		players = s.enrichPlayersWithAllStats(players)
	}

	result := make([]models.RedFlagPlayer, 0)
	for _, p := range players {
		score, formDecline, outputDrop, reasons := calcRedFlag(p)
		if score < 5.0 || len(reasons) == 0 {
			continue
		}
		result = append(result, models.RedFlagPlayer{
			Player:       p,
			RedFlagScore: score,
			FormDecline:  math.Round(formDecline*100) / 100,
			OutputDrop:   math.Round(outputDrop*100) / 100,
			Reasons:      reasons,
		})
	}

	// Sort descending by red flag score
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j].RedFlagScore > result[j-1].RedFlagScore; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	if len(result) > 12 {
		result = result[:12]
	}
	return result, nil
}

// calcBenchwarmer scores a player's consistency/reliability (0-10, higher = better benchwarmer).
func calcBenchwarmer(p models.Player) (score float64, label string) {
	// Form consistency: peaks at 6.5 (solid average), drops off for extremes
	formConsistency := math.Max(0, 10-math.Abs(p.FormScore-6.5)*3)

	// Minutes: reward regular play up to ~1350 mins (15 games × 90)
	minuteScore := math.Min(10, float64(p.MinutesPlayed)/135.0)

	// Output: reward steady but non-elite contributions per 90
	minutes90 := math.Max(1, float64(p.MinutesPlayed)/90.0)
	gA := float64(p.Goals + p.Assists)
	outputPer90 := gA / minutes90

	var outputScore float64
	switch p.Position {
	case "GK":
		outputScore = 6 // keepers are judged on form/minutes only
	case "DEF":
		outputScore = math.Max(0, 10-math.Abs(outputPer90-0.15)*25)
	case "MID":
		outputScore = math.Max(0, 10-math.Abs(outputPer90-0.22)*20)
	default: // FWD
		outputScore = math.Max(0, 10-math.Abs(outputPer90-0.30)*15)
	}

	score = math.Round((formConsistency*0.50+minuteScore*0.30+outputScore*0.20)*100) / 100

	switch {
	case score >= 7:
		label = "Rock Solid"
	case score >= 5:
		label = "Steady Option"
	default:
		label = "Rotation Pick"
	}
	return
}

// GetBenchwarmers returns consistent, reliable non-elite players — good rotation options.
func (s *PredictionService) GetBenchwarmers(league, position, timeFilter string, w models.PredictionWeights) ([]models.BenchwarmerPlayer, error) {
	query := s.db.Model(&models.Player{}).
		Where("league IN ?", supportedLeagueNames()).
		Where("minutes_played >= ?", 270).
		Where("form_score >= ?", 5.5)
	if league != "" {
		query = query.Where("league = ?", league)
	}
	if position != "" {
		query = query.Where("position = ?", position)
	}
	var players []models.Player
	if err := query.Find(&players).Error; err != nil {
		return nil, err
	}

	if timeFilter == "overall" {
		players = s.enrichPlayersWithAllStats(players)
	}

	result := make([]models.BenchwarmerPlayer, 0)
	for _, p := range players {
		pred := s.calcPrediction(p, w)
		// Exclude top players, hidden gems, and red flags
		if pred.PredictedScore >= 7.0 || pred.HiddenGem {
			continue
		}
		rfScore, _, _, _ := calcRedFlag(p)
		if rfScore >= 5.0 {
			continue
		}
		score, label := calcBenchwarmer(p)
		if score < 4.0 {
			continue
		}
		result = append(result, models.BenchwarmerPlayer{
			Player:           p,
			ConsistencyScore: score,
			Label:            label,
		})
	}

	// Sort descending by consistency score
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j].ConsistencyScore > result[j-1].ConsistencyScore; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	if len(result) > 12 {
		result = result[:12]
	}
	return result, nil
}

// GetMomentum returns last 10 real game performances for a player from the API.
func (s *PredictionService) GetMomentum(playerID uint) (*models.MomentumData, error) {
	var player models.Player
	if err := s.db.First(&player, playerID).Error; err != nil {
		return nil, fmt.Errorf("player not found: %w", err)
	}

	stats, err := s.client.GetPlayerStats(playerID)
	if err != nil {
		return nil, fmt.Errorf("could not fetch player stats: %w", err)
	}

	// Take up to the last 10 games
	if len(stats) > 10 {
		stats = stats[len(stats)-10:]
	}

	games := make([]models.MomentumGame, 0, len(stats))
	for _, st := range stats {
		if st.MinutesPlayed == 0 {
			continue
		}

		opponent := st.Event.AwayTeam
		if strings.EqualFold(st.Event.AwayTeam, player.TeamName) {
			opponent = st.Event.HomeTeam
		}

		date := st.Event.EventDate
		if t, err := time.Parse(time.RFC3339, date); err == nil {
			date = t.Format("2006-01-02")
		}

		score := st.Rating
		if score == 0 {
			score = math.Min(10, 6.0+float64(st.Goals)*0.5+float64(st.GoalAssist)*0.3)
		}

		games = append(games, models.MomentumGame{
			Date:     date,
			Opponent: opponent,
			Score:    math.Round(score*10) / 10,
			Goals:    int(st.Goals),
			Assists:  int(st.GoalAssist),
			Minutes:  int(st.MinutesPlayed),
		})
	}

	trend := "stable"
	n := len(games)
	if n >= 2 {
		half := n / 2
		first, last := 0.0, 0.0
		for i := 0; i < half; i++ {
			first += games[i].Score
			last += games[n-half+i].Score
		}
		diff := (last - first) / float64(half)
		if diff > 0.5 {
			trend = "rising"
		} else if diff < -0.5 {
			trend = "falling"
		}
	}

	return &models.MomentumData{
		Player: player,
		Games:  games,
		Trend:  trend,
	}, nil
}

// GetSynergy returns synergy analysis for a set of players.
func (s *PredictionService) GetSynergy(playerIDs []uint, w models.PredictionWeights) (*models.SynergyResult, error) {
	players := make([]models.Player, 0, len(playerIDs))
	for _, id := range playerIDs {
		var p models.Player
		if err := s.db.First(&p, id).Error; err == nil {
			players = append(players, p)
		}
	}

	total := 0.0
	for _, p := range players {
		pred := s.calcPrediction(p, w)
		total += pred.PredictedScore
	}

	positions := map[string]bool{}
	for _, p := range players {
		positions[p.Position] = true
	}
	diversityBonus := float64(len(positions)-1) * 0.5
	synergyScore := total + diversityBonus

	return &models.SynergyResult{
		Players:        players,
		TotalPredicted: math.Round(total*100) / 100,
		SynergyBonus:   math.Round(diversityBonus*100) / 100,
		SynergyScore:   math.Round(synergyScore*100) / 100,
	}, nil
}

// calcPrediction applies the scoring formula to a single player.
func (s *PredictionService) calcPrediction(player models.Player, w models.PredictionWeights) *models.PlayerPrediction {
	total := w.Form + w.Threat + w.Opponent + w.Minutes + w.HomeAway
	if total == 0 {
		w = models.DefaultWeights()
		total = 1
	}
	w.Form /= total
	w.Threat /= total
	w.Opponent /= total
	w.Minutes /= total
	w.HomeAway /= total

	threatRaw := player.XG*1.5 + player.XA*1.2 + float64(player.Goals)*0.8 + float64(player.Assists)*0.6
	threatScore := math.Min(10, threatRaw/5.0)

	rng := rand.New(rand.NewSource(int64(player.ID) * 7))
	opponentDiff := 3.0 + rng.Float64()*7.0
	minutesScore := math.Min(10, float64(player.MinutesPlayed)/90.0)
	homeAwayFactor := 4.0 + rng.Float64()*6.0

	formC := w.Form * player.FormScore
	threatC := w.Threat * threatScore
	oppC := w.Opponent * opponentDiff
	minC := w.Minutes * minutesScore
	haC := w.HomeAway * homeAwayFactor

	predicted := formC + threatC + oppC + minC + haC

	risk := "high"
	if predicted >= 7 {
		risk = "low"
	} else if predicted >= 4.5 {
		risk = "medium"
	}

	hiddenGem := predicted >= 5.5 && risk != "low" && (player.Goals+player.Assists) < 15

	return &models.PlayerPrediction{
		Player:             player,
		PredictedScore:     math.Round(predicted*100) / 100,
		RiskLevel:          risk,
		HiddenGem:          hiddenGem,
		FormContribution:   math.Round(formC*100) / 100,
		ThreatContribution: math.Round(threatC*100) / 100,
		OpponentDifficulty: math.Round(oppC*100) / 100,
		MinutesLikelihood:  math.Round(minC*100) / 100,
		HomeAwayFactor:     math.Round(haC*100) / 100,
	}
}
