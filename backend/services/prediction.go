package services

import (
	"fmt"
	"math"
	"prediplay/backend/bzzoiro"
	"prediplay/backend/models"
	"sort"
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

// targetLeagues maps country name to canonical league name used throughout the app.
var targetLeagues = map[string]string{
	"England": "Premier League",
	"Spain":   "La Liga",
	"Germany": "Bundesliga",
	"Italy":   "Serie A",
	"France":  "Ligue 1",
}

func supportedLeagueNames() []string {
	names := make([]string, 0, len(targetLeagues))
	for _, name := range targetLeagues {
		names = append(names, name)
	}
	return names
}

// ─── Stat aggregation ─────────────────────────────────────────────────────────

// playedGames returns only entries where the player actually took the pitch.
func playedGames(stats []models.PlayerStat) []models.PlayerStat {
	out := make([]models.PlayerStat, 0, len(stats))
	for _, st := range stats {
		if st.MinutesPlayed > 0 {
			out = append(out, st)
		}
	}
	return out
}

// sortByDateDesc sorts stats in-place, most recent game first.
func sortByDateDesc(stats []models.PlayerStat) {
	sort.Slice(stats, func(i, j int) bool {
		di, dj := stats[i].Event.EventDate, stats[j].Event.EventDate
		if len(di) > 10 {
			di = di[:10]
		}
		if len(dj) > 10 {
			dj = dj[:10]
		}
		return di > dj
	})
}

// aggregateOverall computes full-season totals into the Player's main stat fields.
func aggregateOverall(p *models.Player, stats []models.PlayerStat) {
	var mins, goals, assists, shots, shotsOT, keyPasses uint
	var totalPasses, accPasses, duelsWon, duelsTotal uint
	var tacklesWon, tacklesTotal, yellowCards, redCards, saves, gconceded uint
	var xg, xa, rating float64
	var ratedGames, games int

	for _, st := range stats {
		if st.MinutesPlayed > 0 {
			games++
		}
		mins += st.MinutesPlayed
		goals += st.Goals
		assists += st.GoalAssist
		xg += st.ExpectedGoals
		xa += st.ExpectedAssists
		shots += st.TotalShots
		shotsOT += st.ShotsOnTarget
		keyPasses += st.KeyPass
		totalPasses += st.TotalPass
		accPasses += st.AccuratePass
		duelsWon += st.DuelWon
		duelsTotal += st.DuelWon + st.DuelLost
		tacklesWon += st.WonTackle
		tacklesTotal += st.TotalTackle
		yellowCards += st.YellowCard
		redCards += st.RedCard
		saves += st.Saves
		gconceded += st.GoalsConceded
		if st.Rating > 0 {
			rating += st.Rating
			ratedGames++
		}
	}

	p.GamesPlayed = games
	p.MinutesPlayed = int(mins)
	p.Goals = int(goals)
	p.Assists = int(assists)
	p.XG = xg
	p.XA = xa
	p.TotalShots = int(shots)
	p.ShotsOnTarget = int(shotsOT)
	p.KeyPasses = int(keyPasses)
	p.TotalPasses = int(totalPasses)
	p.AccuratePasses = int(accPasses)
	p.DuelsWon = int(duelsWon)
	p.DuelsTotal = int(duelsTotal)
	p.TacklesWon = int(tacklesWon)
	p.TacklesTotal = int(tacklesTotal)
	p.YellowCards = int(yellowCards)
	p.RedCards = int(redCards)
	p.Saves = int(saves)
	p.GoalsConceded = int(gconceded)

	if ratedGames > 0 {
		p.FormScore = rating / float64(ratedGames)
	} else if p.FormScore == 0 {
		p.FormScore = 6.0
	}
}

// aggregateRecent computes stats from the last 3 played games into the Player's Recent* fields.
func aggregateRecent(p *models.Player, stats []models.PlayerStat) {
	var mins, goals, assists, shots, shotsOT, keyPasses uint
	var totalPasses, accPasses, duelsWon, duelsTotal uint
	var tacklesWon, tacklesTotal, yellowCards, redCards, saves, gconceded uint
	var xg, xa, rating float64
	var ratedGames int

	for _, st := range stats {
		mins += st.MinutesPlayed
		goals += st.Goals
		assists += st.GoalAssist
		xg += st.ExpectedGoals
		xa += st.ExpectedAssists
		shots += st.TotalShots
		shotsOT += st.ShotsOnTarget
		keyPasses += st.KeyPass
		totalPasses += st.TotalPass
		accPasses += st.AccuratePass
		duelsWon += st.DuelWon
		duelsTotal += st.DuelWon + st.DuelLost
		tacklesWon += st.WonTackle
		tacklesTotal += st.TotalTackle
		yellowCards += st.YellowCard
		redCards += st.RedCard
		saves += st.Saves
		gconceded += st.GoalsConceded
		if st.Rating > 0 {
			rating += st.Rating
			ratedGames++
		}
	}

	p.RecentMinutes = int(mins)
	p.RecentGoals = int(goals)
	p.RecentAssists = int(assists)
	p.RecentXG = xg
	p.RecentXA = xa
	p.RecentTotalShots = int(shots)
	p.RecentShotsOnTarget = int(shotsOT)
	p.RecentKeyPasses = int(keyPasses)
	p.RecentTotalPasses = int(totalPasses)
	p.RecentAccuratePasses = int(accPasses)
	p.RecentDuelsWon = int(duelsWon)
	p.RecentDuelsTotal = int(duelsTotal)
	p.RecentTacklesWon = int(tacklesWon)
	p.RecentTacklesTotal = int(tacklesTotal)
	p.RecentYellowCards = int(yellowCards)
	p.RecentRedCards = int(redCards)
	p.RecentSaves = int(saves)
	p.RecentGoalsConceded = int(gconceded)

	if ratedGames > 0 {
		p.RecentFormScore = rating / float64(ratedGames)
	} else {
		p.RecentFormScore = 6.0
	}
}

// withRecentStats returns a copy of p with all Recent* fields promoted to the main stat fields.
// Used so scoring functions always read from the same fields regardless of time filter.
func withRecentStats(p models.Player) models.Player {
	p.GamesPlayed = p.RecentGamesPlayed
	p.MinutesPlayed = p.RecentMinutes
	p.Goals = p.RecentGoals
	p.Assists = p.RecentAssists
	p.XG = p.RecentXG
	p.XA = p.RecentXA
	p.TotalShots = p.RecentTotalShots
	p.ShotsOnTarget = p.RecentShotsOnTarget
	p.KeyPasses = p.RecentKeyPasses
	p.TotalPasses = p.RecentTotalPasses
	p.AccuratePasses = p.RecentAccuratePasses
	p.DuelsWon = p.RecentDuelsWon
	p.DuelsTotal = p.RecentDuelsTotal
	p.TacklesWon = p.RecentTacklesWon
	p.TacklesTotal = p.RecentTacklesTotal
	p.YellowCards = p.RecentYellowCards
	p.RedCards = p.RecentRedCards
	p.Saves = p.RecentSaves
	p.GoalsConceded = p.RecentGoalsConceded
	if p.RecentFormScore > 0 {
		p.FormScore = p.RecentFormScore
	}
	return p
}

// scoringView returns the correct stat view of p based on the time filter.
// Players who haven't played in each of the last 3 games are excluded from recent view.
func scoringView(p models.Player, timeFilter string) (models.Player, bool) {
	if timeFilter == "overall" {
		return p, true
	}
	if p.RecentGamesPlayed < 3 {
		return p, false
	}
	return withRecentStats(p), true
}

// ─── Sync ─────────────────────────────────────────────────────────────────────

// SyncPlayers refreshes player and stats data for all 5 supported leagues.
func (s *PredictionService) SyncPlayers() {
	fmt.Println("[sync] Starting player sync…")

	apiLeagues, err := s.client.GetLeagues()
	if err != nil {
		fmt.Printf("[sync] Warning: could not fetch leagues: %v\n", err)
	}
	leagueIDByName := map[string]uint{}
	for _, l := range apiLeagues {
		leagueIDByName[l.Name] = l.ID
	}

	var players []models.Player
	for country, leagueName := range targetLeagues {
		teams, err := s.client.GetTeams(country, leagueIDByName[leagueName])
		if err != nil {
			fmt.Printf("[sync] Warning: teams for %s: %v\n", country, err)
			continue
		}
		for _, team := range teams {
			teamPlayers, err := s.client.GetPlayersFirstPage("", fmt.Sprintf("%d", team.ID))
			if err != nil {
				continue
			}
			for i := range teamPlayers {
				teamPlayers[i].League = leagueName
			}
			players = append(players, teamPlayers...)
		}
	}

	nextOpponentByTeam := map[uint]string{}
	today := time.Now().Format("2006-01-02")
	nextWeek := time.Now().AddDate(0, 0, 14).Format("2006-01-02")
	if events, err := s.client.GetEvents(today, nextWeek, "", ""); err == nil {
		sort.Slice(events, func(i, j int) bool {
			return events[i].Date.Before(events[j].Date)
		})
		for _, ev := range events {
			if _, done := nextOpponentByTeam[ev.HomeTeamID]; !done {
				nextOpponentByTeam[ev.HomeTeamID] = ev.AwayTeam.Name
			}
			if _, done := nextOpponentByTeam[ev.AwayTeamID]; !done {
				nextOpponentByTeam[ev.AwayTeamID] = ev.HomeTeam.Name
			}
		}
	}

	fmt.Printf("[sync] Fetching stats for %d players…\n", len(players))

	const maxConcurrent = 10
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for i := range players {
		wg.Add(1)
		go func(p models.Player) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			s.enrichAndSave(&p, nextOpponentByTeam[p.TeamID])
		}(players[i])
	}

	wg.Wait()
	fmt.Println("[sync] Player sync complete")
}

// playerVsOpponentScore returns a 0-10 score for the player's historical performance
// against a given opponent. Returns 5.0 (neutral) if no prior history exists.
func playerVsOpponentScore(stats []models.PlayerStat, opponentTeamName string) float64 {
	if opponentTeamName == "" {
		return 5.0
	}
	var matched []models.PlayerStat
	for _, st := range stats {
		if st.MinutesPlayed > 0 &&
			(strings.EqualFold(st.Event.HomeTeam, opponentTeamName) ||
				strings.EqualFold(st.Event.AwayTeam, opponentTeamName)) {
			matched = append(matched, st)
		}
	}
	if len(matched) == 0 {
		return 5.0
	}
	total := 0.0
	for _, st := range matched {
		score := st.Rating
		if score == 0 {
			score = 6.0 + float64(st.Goals)*0.5 + float64(st.GoalAssist)*0.3
		}
		total += score
	}
	avg := total / float64(len(matched))
	return math.Min(10, math.Max(1, 5.0+(avg-6.5)*5.0))
}

// enrichAndSave fetches all stats for a player, computes overall + recent aggregates, and upserts to DB.
func (s *PredictionService) enrichAndSave(p *models.Player, nextOpponent string) {
	stats, err := s.client.GetPlayerStats(p.ID)
	if err != nil || len(stats) == 0 {
		s.db.Save(p)
		return
	}

	aggregateOverall(p, stats)

	played := playedGames(stats)
	sortByDateDesc(played)
	if len(played) > 3 {
		played = played[:3]
	}
	p.RecentGamesPlayed = len(played)
	aggregateRecent(p, played)

	p.NextOpponent = nextOpponent
	p.OpponentScore = playerVsOpponentScore(stats, nextOpponent)

	s.db.Save(p)
}

// ─── Queries ──────────────────────────────────────────────────────────────────

func (s *PredictionService) GetPlayer(playerID uint) (models.Player, error) {
	var p models.Player
	return p, s.db.First(&p, playerID).Error
}

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
	return players, query.Find(&players).Error
}

func (s *PredictionService) GetPlayerPrediction(playerID uint, w models.PredictionWeights) (*models.PlayerPrediction, error) {
	var player models.Player
	if err := s.db.First(&player, playerID).Error; err != nil {
		return nil, fmt.Errorf("player not found: %w", err)
	}
	stats, err := s.client.GetPlayerStats(playerID)
	if err == nil && len(stats) > 0 {
		aggregateOverall(&player, stats)
		s.db.Save(&player)
	}
	return s.calcPrediction(player, w), nil
}

// ─── Top predictions ──────────────────────────────────────────────────────────

// positionQuota defines how many top players to pick per position when no
// position filter is active. Matches a typical attacking lineup shape (4-3-3 / 4-2-3-1).
var positionQuota = map[string]int{
	"GK":  1,
	"DEF": 2,
	"MID": 3,
	"FWD": 3,
}

// pickTopWithPositionQuota selects the best players using per-position quotas so
// no single position can flood the top-N list. Each position competes within itself.
func pickTopWithPositionQuota(preds []models.PlayerPrediction) []models.PlayerPrediction {
	byPos := map[string][]models.PlayerPrediction{
		"GK": {}, "DEF": {}, "MID": {}, "FWD": {},
	}
	for _, p := range preds {
		pos := p.Player.Position
		if _, ok := byPos[pos]; !ok {
			pos = "FWD" // fallback for unexpected position values
		}
		byPos[pos] = append(byPos[pos], p)
	}
	for pos := range byPos {
		sort.Slice(byPos[pos], func(i, j int) bool {
			return byPos[pos][i].PredictedScore > byPos[pos][j].PredictedScore
		})
	}
	result := make([]models.PlayerPrediction, 0, 9)
	for pos, quota := range positionQuota {
		group := byPos[pos]
		if len(group) > quota {
			group = group[:quota]
		}
		result = append(result, group...)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].PredictedScore > result[j].PredictedScore
	})
	return result
}

func (s *PredictionService) GetTopPredictions(league, position, gemFilter, timeFilter string) ([]models.PlayerPrediction, error) {
	players, err := s.loadPlayers(league, position)
	if err != nil {
		return nil, err
	}

	w := models.DefaultWeights()
	preds := make([]models.PlayerPrediction, 0, len(players))
	for _, p := range players {
		scoring, ok := scoringView(p, timeFilter)
		if !ok {
			continue
		}
		pred := s.calcPrediction(scoring, w)
		switch gemFilter {
		case "gems":
			if !pred.HiddenGem {
				continue
			}
		case "non-gems":
			if pred.HiddenGem {
				continue
			}
		}
		preds = append(preds, *pred)
	}

	// When no position filter is set, use a per-position quota so one position
	// can't dominate the list (e.g. GKs whose availability/discipline is always ~10).
	if position == "" {
		return pickTopWithPositionQuota(preds), nil
	}

	sort.Slice(preds, func(i, j int) bool {
		return preds[i].PredictedScore > preds[j].PredictedScore
	})
	if len(preds) > 9 {
		preds = preds[:9]
	}
	return preds, nil
}

// ─── Red flags ────────────────────────────────────────────────────────────────

// GetRedFlags passes the original (full-stat) player to calcRedFlag so it can
// compare recent vs overall as a true decline signal. The scoringView eligibility
// filter (3 games played) is applied manually.
func (s *PredictionService) GetRedFlags(league, position, timeFilter string) ([]models.RedFlagPlayer, error) {
	minMinutes := 270
	if timeFilter != "overall" {
		minMinutes = 45
	}
	players, err := s.loadPlayersMinMinutes(league, position, minMinutes)
	if err != nil {
		return nil, err
	}

	result := make([]models.RedFlagPlayer, 0)
	for _, p := range players {
		if timeFilter != "overall" && p.RecentGamesPlayed < 3 {
			continue
		}
		score, formDecline, outputDrop, reasons := calcRedFlag(p)
		if score < 4.0 || len(reasons) == 0 {
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

	sort.Slice(result, func(i, j int) bool {
		return result[i].RedFlagScore > result[j].RedFlagScore
	})
	if len(result) > 9 {
		result = result[:9]
	}
	return result, nil
}

// ─── Benchwarmers ─────────────────────────────────────────────────────────────

func (s *PredictionService) GetBenchwarmers(league, position, timeFilter string) ([]models.BenchwarmerPlayer, error) {
	minMinutes := 270
	if timeFilter != "overall" {
		minMinutes = 45
	}
	players, err := s.loadPlayersMinMinutes(league, position, minMinutes)
	if err != nil {
		return nil, err
	}

	w := models.DefaultWeights()
	result := make([]models.BenchwarmerPlayer, 0)
	for _, p := range players {
		scoring, ok := scoringView(p, timeFilter)
		if !ok {
			continue
		}
		pred := s.calcPrediction(scoring, w)
		if pred.PredictedScore >= 7.0 || pred.HiddenGem {
			continue
		}
		// Exclude players already flagged as red flags (pass original for decline analysis)
		rfScore, _, _, _ := calcRedFlag(p)
		if rfScore >= 4.0 {
			continue
		}
		score, label := calcBenchwarmer(scoring)
		if score < 4.0 || label == "" {
			continue
		}
		result = append(result, models.BenchwarmerPlayer{
			Player:           scoring,
			ConsistencyScore: score,
			Label:            label,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ConsistencyScore > result[j].ConsistencyScore
	})
	if len(result) > 9 {
		result = result[:9]
	}
	return result, nil
}

// ─── Dashboard ────────────────────────────────────────────────────────────────

var dashboardLeagueList = []string{"Premier League", "La Liga", "Bundesliga", "Serie A", "Ligue 1"}

func (s *PredictionService) GetDashboard(timeFilter string) ([]models.DashboardLeague, error) {
	type leagueResult struct {
		idx      int
		top      []models.PlayerPrediction
		redFlags []models.RedFlagPlayer
	}
	ch := make(chan leagueResult, len(dashboardLeagueList))
	for i, league := range dashboardLeagueList {
		go func(idx int, l string) {
			top, _ := s.GetTopPredictions(l, "", "", timeFilter)
			flags, _ := s.GetRedFlags(l, "", timeFilter)
			if len(top) > 3 {
				top = top[:3]
			}
			if len(flags) > 3 {
				flags = flags[:3]
			}
			ch <- leagueResult{idx: idx, top: top, redFlags: flags}
		}(i, league)
	}
	results := make([]models.DashboardLeague, len(dashboardLeagueList))
	for i, name := range dashboardLeagueList {
		results[i].Name = name
	}
	for range dashboardLeagueList {
		r := <-ch
		results[r.idx].TopPlayers = r.top
		results[r.idx].RedFlags = r.redFlags
	}
	return results, nil
}

// ─── Momentum ─────────────────────────────────────────────────────────────────

func (s *PredictionService) GetMomentum(playerID uint) (*models.MomentumData, error) {
	var player models.Player
	if err := s.db.First(&player, playerID).Error; err != nil {
		return nil, fmt.Errorf("player not found: %w", err)
	}

	stats, _ := s.client.GetPlayerStats(playerID)

	played := playedGames(stats)
	sortByDateDesc(played)
	if len(played) > 10 {
		played = played[:10]
	}

	games := make([]models.MomentumGame, 0, len(played))
	for _, st := range played {
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
		recent, older := 0.0, 0.0
		for i := 0; i < half; i++ {
			recent += games[i].Score
			older += games[n-half+i].Score
		}
		diff := (recent - older) / float64(half)
		if diff > 0.5 {
			trend = "rising"
		} else if diff < -0.5 {
			trend = "falling"
		}
	} else if player.RecentFormScore > 0 && player.FormScore > 0 {
		diff := player.RecentFormScore - player.FormScore
		if diff > 0.5 {
			trend = "rising"
		} else if diff < -0.5 {
			trend = "falling"
		}
	}

	return &models.MomentumData{Player: player, Games: games, Trend: trend}, nil
}

// ─── Synergy ──────────────────────────────────────────────────────────────────

func (s *PredictionService) GetSynergy(playerIDs []uint, w models.PredictionWeights) (*models.SynergyResult, error) {
	players := make([]models.Player, 0, len(playerIDs))
	for _, id := range playerIDs {
		var p models.Player
		if err := s.db.First(&p, id).Error; err == nil {
			players = append(players, p)
		}
	}
	total := 0.0
	positions := map[string]bool{}
	for _, p := range players {
		total += s.calcPrediction(p, w).PredictedScore
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

// ─── DB helpers ───────────────────────────────────────────────────────────────

func (s *PredictionService) loadPlayers(league, position string) ([]models.Player, error) {
	query := s.db.Model(&models.Player{})
	if league != "" {
		query = query.Where("league = ?", league)
	} else {
		query = query.Where("league IN ?", supportedLeagueNames())
	}
	if position != "" {
		query = query.Where("position = ?", position)
	}
	var players []models.Player
	return players, query.Find(&players).Error
}

func (s *PredictionService) loadPlayersMinMinutes(league, position string, minMinutes int) ([]models.Player, error) {
	query := s.db.Model(&models.Player{}).Where("minutes_played >= ?", minMinutes)
	if league != "" {
		query = query.Where("league = ?", league)
	} else {
		query = query.Where("league IN ?", supportedLeagueNames())
	}
	if position != "" {
		query = query.Where("position = ?", position)
	}
	var players []models.Player
	return players, query.Find(&players).Error
}

// ─── Scoring components (all return 0-10) ─────────────────────────────────────
//
// Every component is calibrated so that a genuinely elite player scores ~8-10,
// a solid average player scores ~5-6, and a poor player scores ~2-3.
// This spread prevents scores from clustering in the 5.5-6.5 band.

// formComponent passes the API match rating through directly (scale 1-10).
// 6.5 is the neutral baseline; most players cluster 6.0-7.5.
func formComponent(p models.Player) float64 {
	f := p.FormScore
	if f <= 0 {
		return 6.0
	}
	return math.Max(0, math.Min(10, f))
}

// attackComponent measures goal-scoring threat with three independent signals:
//
//  1. xG per 90 — quality of positions taken (the primary predictive signal).
//     xG is more stable than actual goals and avoids penalising unlucky players.
//     Elite FWD ≈ 0.55 xG/90 → score ≈ 7.7; average MID ≈ 0.10 → score ≈ 1.4.
//
//  2. Conversion bonus — goals scored above expected (goals/90 − xG/90), capped
//     at +2. This rewards clinical finishers without double-counting xG quality.
//     A player scoring exactly to their xG gets 0 bonus; a 0.2/90 over-performer
//     gets +0.8.
//
//  3. Shot on target volume per 90 — how often the player tests the keeper,
//     independent of whether those shots were "expected". Capped at +2.
//     Elite FWD ≈ 2.5 SoT/90 → +1.5; average ≈ 1.0 SoT/90 → +0.6.
func attackComponent(p models.Player) float64 {
	mins90 := math.Max(1, float64(p.MinutesPlayed)/90.0)
	xgPer90 := p.XG / mins90
	goalsPer90 := float64(p.Goals) / mins90

	xgScore := math.Min(8, xgPer90*14)
	conversionBonus := math.Min(2, math.Max(0, goalsPer90-xgPer90)*4)
	sotPer90 := float64(p.ShotsOnTarget) / mins90
	shotVolume := math.Min(2, sotPer90*0.6)

	return math.Min(10, xgScore+conversionBonus+shotVolume)
}

// creativityComponent measures chance creation with three independent signals:
//
//  1. xA per 90 — quality of chances created (the primary predictive signal).
//     Elite playmaker ≈ 0.35 xA/90 → score ≈ 5.6; average MID ≈ 0.10 → ~1.6.
//
//  2. Assist delivery bonus — assists/90 above xA/90 (over-delivering on chances).
//     Rewards players whose team-mates convert their passes well. Capped at +2.
//
//  3. Key pass quality — xA divided by total key passes.
//     A high xA-per-key-pass means dangerous passes into the box, not volume of
//     speculative long balls. Elite: 0.12 xA/KP → +2; average: 0.05 xA/KP → +1.
//     Using quality over volume prevents high-turnover players from scoring well.
func creativityComponent(p models.Player) float64 {
	mins90 := math.Max(1, float64(p.MinutesPlayed)/90.0)
	xaPer90 := p.XA / mins90
	assistsPer90 := float64(p.Assists) / mins90

	xaScore := math.Min(8, xaPer90*16)
	assistBonus := math.Min(2, math.Max(0, assistsPer90-xaPer90)*4)
	kpQuality := 0.0
	if p.KeyPasses > 0 {
		xaPerKP := p.XA / float64(p.KeyPasses)
		kpQuality = math.Min(2, xaPerKP*20) // 0.10 xA/KP → 2.0
	}

	return math.Min(10, xaScore+assistBonus+kpQuality)
}

// defensiveComponent is fully position-specific:
//
//   - GK:  two signals combined — save rate (0-8) + save volume per game (0-2).
//     Rate alone rewards a GK who faces only 1 shot and saves it; volume ensures
//     busy, reliable GKs score higher than those with easy games.
//     Save rate: league-average PL GK ≈72% → ~5.7; 80% → ~7.4; 85% → ~8.
//     Volume bonus: 4 saves/game → +2; 2 saves/game → +1.
//
//   - DEF: duel win rate (50% weight) + tackle win rate (50%) using quality signals,
//     PLUS a volume bonus for high-activity defenders (tackles per 90).
//     A DEF winning 60% of 20 duels per game outscores one winning 100% of 2.
//
//   - MID: duel win rate only (pressing and ball-winning), with a floor of 1.5
//     so creative MIDs who avoid duels aren't unfairly penalised.
//
//   - FWD: minimal duel contribution for hold-up play. Score range 1–6.
func defensiveComponent(p models.Player) float64 {
	mins90 := math.Max(1, float64(p.MinutesPlayed)/90.0)

	switch p.Position {
	case "GK":
		total := float64(p.Saves + p.GoalsConceded)
		if total < 3 {
			return 6.0 // insufficient data — neutral
		}
		saveRate := float64(p.Saves) / total
		// Rate component (0-8): 50%→0, 72%(avg)→5.7, 80%→7.4, 85%→8
		rateScore := math.Max(0, math.Min(8, (saveRate-0.50)/0.35*8))
		// Volume bonus (0-2): elite GK makes 3-5 saves/game; rewards busy, reliable GKs
		games := math.Max(1, float64(p.GamesPlayed))
		savesPerGame := float64(p.Saves) / games
		volumeBonus := math.Min(2, savesPerGame/4.0*2)
		return math.Min(10, rateScore+volumeBonus)

	case "DEF":
		duelRate := 0.50
		if p.DuelsTotal > 0 {
			duelRate = float64(p.DuelsWon) / float64(p.DuelsTotal)
		}
		tackleRate := 0.50
		if p.TacklesTotal > 0 {
			tackleRate = float64(p.TacklesWon) / float64(p.TacklesTotal)
		}
		// Volume bonus: high-activity defenders rewarded (cap at +1.5)
		// Elite ball-winning DEF makes 4-6 tackles/90
		tacklesPer90 := float64(p.TacklesTotal) / mins90
		activityBonus := math.Min(1.5, tacklesPer90/4.0*1.5)
		// Quality rates (0-5 each) + volume bonus
		return math.Min(10, duelRate*5+tackleRate*5+activityBonus)

	case "MID":
		duelRate := 0.50
		if p.DuelsTotal > 0 {
			duelRate = float64(p.DuelsWon) / float64(p.DuelsTotal)
		}
		return math.Min(10, duelRate*7+1.5)

	default: // FWD — hold-up play contribution only
		duelRate := 0.50
		if p.DuelsTotal > 0 {
			duelRate = float64(p.DuelsWon) / float64(p.DuelsTotal)
		}
		return math.Min(10, duelRate*5+1.0)
	}
}

// availabilityComponent scores average minutes per game vs a full 90.
// A player averaging 90 min/game scores 10; 45 min/game scores 5.
func availabilityComponent(p models.Player) float64 {
	games := math.Max(1, float64(p.GamesPlayed))
	avgMins := float64(p.MinutesPlayed) / games
	return math.Min(10, avgMins/90.0*10)
}

// disciplineComponent penalises cards. Starting at 10:
// each yellow card per game costs 5 points; each red costs 15.
func disciplineComponent(p models.Player) float64 {
	games := math.Max(1, float64(p.GamesPlayed))
	yellowPerGame := float64(p.YellowCards) / games
	redPerGame := float64(p.RedCards) / games
	return math.Max(0, 10-yellowPerGame*5-redPerGame*15)
}

// ─── Prediction ───────────────────────────────────────────────────────────────

// calcPrediction combines seven independent components with position-specific weights.
//
// Weight rationale per position:
//
//	GK:  Defensive dominates (0.60) — save rate + volume is the primary signal.
//	     Availability and discipline weights are kept low: GKs almost always play
//	     full 90 and rarely get cards, so these components have very low variance
//	     and would inflate all GK scores artificially if weighted too high.
//	     Weights: form=0.22, def=0.60, avail=0.06, disc=0.04, opp=0.08  → sum 1.00
//
//	DEF: Defensive work is the primary job (0.30). Form is holistic (0.22).
//	     Attacking output matters for DEF (set-pieces, overlapping runs): 0.15 combined.
//	     Opponent weight meaningful (facing elite FWDs = harder to shine): 0.10.
//	     Weights: form=0.22, atk=0.08, cre=0.07, def=0.30, avail=0.13, disc=0.10, opp=0.10 → 1.00
//
//	MID: Most balanced role. Creativity is highest single weight (0.24) reflecting
//	     the modern box-to-box and #10 importance. Attack and defensive both relevant.
//	     Weights: form=0.20, atk=0.17, cre=0.24, def=0.12, avail=0.10, disc=0.09, opp=0.08 → 1.00
//
//	FWD: Attack dominates (0.33). Creativity significant (link-up, assists): 0.17.
//	     Opponent weight raised to 0.12 — strikers are the most affected by defensive
//	     quality: facing a top-4 defence vs a relegation candidate is huge.
//	     Defensive contribution minimal (0.02): hold-up play, not tracked well by stats.
//	     Weights: form=0.18, atk=0.33, cre=0.17, def=0.02, avail=0.10, disc=0.08, opp=0.12 → 1.00
func (s *PredictionService) calcPrediction(player models.Player, _ models.PredictionWeights) *models.PlayerPrediction {
	form := formComponent(player)
	attack := attackComponent(player)
	creativity := creativityComponent(player)
	defensive := defensiveComponent(player)
	availability := availabilityComponent(player)
	discipline := disciplineComponent(player)
	opponent := player.OpponentScore
	if opponent == 0 {
		opponent = 5.0
	}

	var predicted float64
	switch player.Position {
	case "GK":
		// avail and disc weights deliberately low: GKs almost always play 90 mins
		// and rarely get cards — high-weight near-constants inflate all GK scores.
		predicted = form*0.22 + defensive*0.60 + availability*0.06 + discipline*0.04 + opponent*0.08
	case "DEF":
		predicted = form*0.22 + attack*0.08 + creativity*0.07 + defensive*0.30 +
			availability*0.13 + discipline*0.10 + opponent*0.10
	case "MID":
		predicted = form*0.20 + attack*0.17 + creativity*0.24 + defensive*0.12 +
			availability*0.10 + discipline*0.09 + opponent*0.08
	default: // FWD
		predicted = form*0.18 + attack*0.33 + creativity*0.17 + defensive*0.02 +
			availability*0.10 + discipline*0.08 + opponent*0.12
	}
	predicted = math.Round(predicted*100) / 100

	risk := "high"
	if predicted >= 7.0 {
		risk = "low"
	} else if predicted >= 4.5 {
		risk = "medium"
	}

	hiddenGem := isHiddenGem(player, predicted, attack, creativity)

	return &models.PlayerPrediction{
		Player:             player,
		PredictedScore:     predicted,
		RiskLevel:          risk,
		HiddenGem:          hiddenGem,
		FormContribution:   math.Round(form*100) / 100,
		ThreatContribution: math.Round(attack*100) / 100,
		OpponentDifficulty: math.Round(opponent*100) / 100,
		MinutesLikelihood:  math.Round(availability*100) / 100,
		HomeAwayFactor:     math.Round(defensive*100) / 100,
	}
}

// isHiddenGem identifies players whose underlying metrics significantly outpace
// their visible returns — suggesting untapped potential or a breakout incoming.
//
// Requirements: predicted score in the 4.5-8.0 band (not too weak, not already elite)
// AND fewer than 12 combined G+A (not yet well-known or expensively priced in).
//
// Six independent signals — any single one qualifies the player:
//
//  1. xG+xA/90 outpaces actual G+A/90 by ≥40%: underlying quality is real,
//     luck or finishing efficiency hasn't caught up. Regression to mean favours them.
//
//  2. High shot volume with below-average conversion vs shots on target:
//     consistently getting into shooting positions; finishing will click.
//
//  3. High creativity score but very few assists relative to key passes:
//     team-mates are spurning the chances, not the player. Assists due.
//
//  4. Strong attack component but fewer than 6 total returns:
//     quality threat not yet visible in the stat line (new team, early season etc).
//
//  5. High xG per shot (≥0.12): the player takes shots from premium positions
//     (inside the box, one-on-ones) but hasn't scored yet. Quality > quantity.
//
//  6. Improving trajectory: recent xG+xA/90 is ≥30% above the season average,
//     meaning underlying form is genuinely trending up right now.
func isHiddenGem(p models.Player, predicted, attackScore, creativityScore float64) bool {
	if predicted < 4.5 || predicted >= 8.0 {
		return false
	}
	if p.Goals+p.Assists >= 12 {
		return false // already well-known / priced in
	}
	mins90 := math.Max(1, float64(p.MinutesPlayed)/90.0)
	xgXaPer90 := (p.XG + p.XA) / mins90
	gAPer90 := float64(p.Goals+p.Assists) / mins90

	// Signal 1: expected stats far exceed actual returns
	underperformingExpected := xgXaPer90 >= 0.25 && xgXaPer90 > gAPer90*1.40

	// Signal 2: high shot volume, conversion hasn't clicked yet
	prolificShooter := p.TotalShots >= 15 && p.ShotsOnTarget > 0 &&
		float64(p.Goals) < float64(p.ShotsOnTarget)*0.22

	// Signal 3: creative output not rewarded with assists
	creativeButUnrewarded := creativityScore >= 5.0 && p.KeyPasses > 0 &&
		float64(p.Assists) < float64(p.KeyPasses)*0.15

	// Signal 4: genuine attack threat, returns not there yet
	highThreatLowReturns := attackScore >= 5.0 && p.Goals+p.Assists < 6

	// Signal 5: taking high-quality shots but not converting yet
	highQualityPositions := false
	if p.TotalShots >= 6 {
		xgPerShot := p.XG / float64(p.TotalShots)
		highQualityPositions = xgPerShot >= 0.12 && p.Goals < 3
	}

	// Signal 6: recent underlying stats trending clearly upward
	improvingTrajectory := false
	if p.RecentGamesPlayed >= 3 && p.RecentMinutes > 0 && xgXaPer90 > 0.05 {
		recentXT90 := (p.RecentXG + p.RecentXA) / math.Max(1, float64(p.RecentMinutes)/90.0)
		improvingTrajectory = recentXT90 > xgXaPer90*1.30
	}

	return underperformingExpected || prolificShooter || creativeButUnrewarded ||
		highThreatLowReturns || highQualityPositions || improvingTrajectory
}

// ─── Red flags ────────────────────────────────────────────────────────────────

// calcRedFlag always receives the full player (both overall and recent stats intact)
// so it can detect true decline rather than just absolute badness.
//
// Eight signals are computed and combined with position-specific weights.
// The composite produces a 0-10 alarm score; ≥4.0 is shown to users.
//
// FormDecline calibration rationale:
//
//	absFormBad uses 7.0 as the "good form" baseline (not 6.5).
//	Formula: (7.0 - recentForm) / 3.0 * 10 — so:
//	  recentForm 7.0 → 0 (fine), 6.0 → 3.3, 5.0 → 6.7 (alarming), 4.0 → 10.
//	The old formula using 6.5 as baseline gave recentForm=5.0 only 2.3 — far too lenient.
//
//	relFormDecline uses an absolute-point scale: each rating point dropped scores 2.5.
//	A 1-point drop (e.g. 7.5 → 6.5) = 2.5; a 2-point drop = 5.0.
//	Old formula divided by season average, meaning a drop from 7.5 to 6.5 scored 1.3 — too lenient.
func calcRedFlag(p models.Player) (score, formDecline, outputDrop float64, reasons []string) {
	mins90 := math.Max(1, float64(p.MinutesPlayed)/90.0)
	recentMins90 := math.Max(0.5, float64(p.RecentMinutes)/90.0)

	// ── 1. Form decline ───────────────────────────────────────────────────────
	overallForm := p.FormScore
	if overallForm <= 0 {
		overallForm = 6.0
	}
	recentForm := p.RecentFormScore
	if recentForm <= 0 {
		recentForm = 6.0
	}
	// Absolute: distance below 7.0 "good form" baseline — calibrated so 5.0 = alarming (6.7)
	absFormBad := math.Max(0, (7.0-recentForm)/3.0*10)
	// Relative: each rating point dropped scores 2.5 (1pt drop=2.5, 2pt=5.0, 4pt=10)
	relFormDecline := 0.0
	if overallForm > recentForm {
		relFormDecline = math.Min(10, (overallForm-recentForm)*2.5)
	}
	formDecline = math.Min(10, math.Max(absFormBad, relFormDecline))

	// ── 2. Attacking output drop ──────────────────────────────────────────────
	overallGA90 := float64(p.Goals+p.Assists) / mins90
	recentGA90 := float64(p.RecentGoals+p.RecentAssists) / recentMins90

	var posBaseline float64
	switch p.Position {
	case "GK":
		posBaseline = 0
	case "DEF":
		posBaseline = 0.10
	case "MID":
		posBaseline = 0.22
	default:
		posBaseline = 0.35
	}
	absOutputBad := 0.0
	if p.Position != "GK" && posBaseline > 0 {
		absOutputBad = math.Max(0, (posBaseline-recentGA90)/posBaseline*9)
	}
	relOutputDecline := 0.0
	if overallGA90 > 0.06 {
		relOutputDecline = math.Max(0, (overallGA90-recentGA90)/overallGA90*10)
	}
	outputDrop = math.Min(10, math.Max(absOutputBad, relOutputDecline))

	// ── 3. Expected-threat decline (xG+xA per 90) ────────────────────────────
	overallXT90 := (p.XG + p.XA) / mins90
	recentXT90 := (p.RecentXG + p.RecentXA) / recentMins90
	xThreatDecline := 0.0
	if p.Position != "GK" && overallXT90 > 0.06 {
		xThreatDecline = math.Max(0, (overallXT90-recentXT90)/overallXT90*10)
	}

	// ── 4. Shot accuracy decline ──────────────────────────────────────────────
	shotAccDecline := 0.0
	if p.TotalShots >= 10 && p.RecentTotalShots > 0 && p.Position != "GK" {
		overallShotAcc := float64(p.ShotsOnTarget) / float64(p.TotalShots)
		recentShotAcc := float64(p.RecentShotsOnTarget) / float64(p.RecentTotalShots)
		if overallShotAcc > 0.10 {
			shotAccDecline = math.Max(0, (overallShotAcc-recentShotAcc)/overallShotAcc*10)
		}
	}

	// ── 5. Passing / involvement decline ─────────────────────────────────────
	involvementDecline := 0.0
	if p.TotalPasses >= 20 && p.RecentTotalPasses > 0 {
		overallPassAcc := float64(p.AccuratePasses) / float64(p.TotalPasses)
		recentPassAcc := float64(p.RecentAccuratePasses) / float64(p.RecentTotalPasses)
		if overallPassAcc > 0.50 {
			involvementDecline = math.Max(0, (overallPassAcc-recentPassAcc)/overallPassAcc*9)
		}
	}

	// ── 6. Discipline risk (recent period only) ───────────────────────────────
	disciplineRisk := math.Min(10, float64(p.RecentYellowCards)*2.5+float64(p.RecentRedCards)*8.0)

	// ── 7. GK-specific: save rate and goals conceded ──────────────────────────
	gkDecline := 0.0
	if p.Position == "GK" {
		overallGKTotal := float64(p.Saves + p.GoalsConceded)
		recentGKTotal := float64(p.RecentSaves + p.RecentGoalsConceded)
		if overallGKTotal >= 5 && recentGKTotal >= 1 {
			overallSaveRate := float64(p.Saves) / overallGKTotal
			recentSaveRate := float64(p.RecentSaves) / recentGKTotal
			if overallSaveRate > 0.40 {
				gkDecline = math.Max(0, (overallSaveRate-recentSaveRate)/overallSaveRate*10)
			}
		}
		gcPerGame := float64(p.RecentGoalsConceded) / math.Max(1, float64(p.RecentGamesPlayed))
		if gcPerGame >= 2.5 {
			gkDecline = math.Max(gkDecline, math.Min(10, (gcPerGame-1.5)*4))
		}
	}

	// ── Composite (position-weighted) ────────────────────────────────────────
	switch p.Position {
	case "GK":
		score = formDecline*0.25 + gkDecline*0.45 + disciplineRisk*0.10 + involvementDecline*0.20
	case "DEF":
		score = formDecline*0.25 + outputDrop*0.18 + xThreatDecline*0.12 +
			involvementDecline*0.22 + disciplineRisk*0.23
	case "MID":
		score = formDecline*0.25 + outputDrop*0.20 + xThreatDecline*0.18 +
			shotAccDecline*0.10 + involvementDecline*0.17 + disciplineRisk*0.10
	default: // FWD
		score = formDecline*0.25 + outputDrop*0.28 + xThreatDecline*0.20 +
			shotAccDecline*0.15 + involvementDecline*0.08 + disciplineRisk*0.04
	}
	score = math.Round(math.Min(10, score)*100) / 100

	// ── Reason strings ────────────────────────────────────────────────────────
	// Thresholds are lower than the old version to surface real concerns earlier.
	if formDecline >= 6.5 {
		reasons = append(reasons, "Form has collapsed")
	} else if formDecline >= 3.5 {
		reasons = append(reasons, "Noticeable form decline")
	} else if recentForm < 6.0 {
		reasons = append(reasons, "Below-average recent form")
	}

	if p.Position != "GK" {
		if outputDrop >= 7 {
			reasons = append(reasons, "Output has completely dried up")
		} else if outputDrop >= 4 {
			reasons = append(reasons, "Significant drop in goal/assist returns")
		}
		if xThreatDecline >= 5 {
			reasons = append(reasons, "xG+xA contribution sharply down")
		}
		if shotAccDecline >= 5 {
			reasons = append(reasons, "Shot accuracy falling off")
		}
		if p.RecentGoals+p.RecentAssists == 0 && p.RecentMinutes >= 180 {
			reasons = append(reasons, "No returns across last 3 games")
		}
		// Specific to attacking positions: not even testing the keeper
		if p.RecentShotsOnTarget == 0 && p.RecentMinutes >= 180 &&
			(p.Position == "FWD" || p.Position == "MID") {
			reasons = append(reasons, "Zero shots on target in last 3 games")
		}
	}
	if p.Position == "GK" && gkDecline >= 4 {
		reasons = append(reasons, "Save rate declining / conceding more heavily")
	}
	if involvementDecline >= 5 {
		reasons = append(reasons, "Fading involvement in build-up play")
	}
	if disciplineRisk >= 4 {
		reasons = append(reasons, "Discipline concerns — risk of suspension")
	}

	return
}

// ─── Benchwarmers scoring ─────────────────────────────────────────────────────

// calcBenchwarmer rewards consistency over brilliance. Five components:
//
//  1. Availability  — average minutes per game vs a full 90.
//
//  2. Form consistency — two sub-signals combined 60/40:
//     a. Band score: how close is the season average to the 6.0-7.5 "reliable" band?
//        A player averaging 6.8 scores near 10; one averaging 8.5 or 4.5 scores low.
//     b. Stability score: how close is the recent form to the season average?
//        |FormScore - RecentFormScore| × 4, inverted. Penalises volatile players.
//        A player at 6.8 overall but 4.5 recently is NOT a benchwarmer — they're declining.
//        This signal directly catches what the band check alone misses.
//
//  3. Output reliability — G+A per 90 proximity to a moderate position baseline.
//     Being too far above OR below the baseline reduces score (benchwarmers are steady, not elite).
//
//  4. Passing reliability — pass accuracy proximity to a position-specific target.
//     Reliable players circulate the ball cleanly without errors or heroics.
//
//  5. Discipline — card rate per game (reliable players stay on the pitch).
func calcBenchwarmer(p models.Player) (score float64, label string) {
	games := math.Max(1, float64(p.GamesPlayed))
	mins90 := math.Max(1, float64(p.MinutesPlayed)/90.0)

	// 1. Availability (0-10)
	avgMins := float64(p.MinutesPlayed) / games
	availScore := math.Min(10, avgMins/90.0*10)

	// 2. Form consistency (0-10) — band score + stability score
	form := p.FormScore
	if form <= 0 {
		form = 6.0
	}
	// 2a. Band: distance from 6.75 target (the centre of the reliable 6.0-7.5 band)
	bandScore := math.Max(0, 10-math.Abs(form-6.75)*3.5)
	// 2b. Stability: recent form matches season average (volatile = unreliable)
	stabilityScore := 10.0 // default full if no recent data
	if p.RecentGamesPlayed >= 3 && p.RecentFormScore > 0 {
		recentForm := p.RecentFormScore
		stabilityScore = math.Max(0, 10-math.Abs(form-recentForm)*4)
	}
	formConsistency := bandScore*0.60 + stabilityScore*0.40

	// 3. Output reliability (0-10) — how close to a moderate, steady baseline
	ga90 := float64(p.Goals+p.Assists) / mins90
	var outputReliability float64
	switch p.Position {
	case "GK":
		total := float64(p.Saves + p.GoalsConceded)
		if total >= 3 {
			saveRate := float64(p.Saves) / total
			outputReliability = math.Min(10, saveRate*10)
		} else {
			outputReliability = 6.0
		}
	case "DEF":
		outputReliability = math.Max(0, 10-math.Abs(ga90-0.10)*30)
	case "MID":
		outputReliability = math.Max(0, 10-math.Abs(ga90-0.20)*22)
	default: // FWD
		outputReliability = math.Max(0, 10-math.Abs(ga90-0.30)*18)
	}

	// 4. Passing reliability (0-10) — accuracy close to position baseline
	passReliability := 6.0
	if p.TotalPasses >= 10 {
		passAcc := float64(p.AccuratePasses) / float64(p.TotalPasses)
		var target float64
		switch p.Position {
		case "GK":
			target = 0.60
		case "DEF":
			target = 0.78
		case "MID":
			target = 0.82
		default:
			target = 0.72
		}
		passReliability = math.Max(0, 10-math.Abs(passAcc-target)*22)
	}

	// 5. Discipline (0-10)
	yellowPerGame := float64(p.YellowCards) / games
	redPerGame := float64(p.RedCards) / games
	discipline := math.Max(0, 10-yellowPerGame*5-redPerGame*15)

	// Weighted composite by position
	switch p.Position {
	case "GK":
		score = availScore*0.30 + formConsistency*0.25 + outputReliability*0.30 + discipline*0.15
	case "DEF":
		score = availScore*0.25 + formConsistency*0.25 + outputReliability*0.18 +
			passReliability*0.17 + discipline*0.15
	case "MID":
		score = availScore*0.20 + formConsistency*0.25 + outputReliability*0.23 +
			passReliability*0.22 + discipline*0.10
	default: // FWD
		score = availScore*0.20 + formConsistency*0.25 + outputReliability*0.30 +
			passReliability*0.15 + discipline*0.10
	}
	score = math.Round(score*100) / 100

	switch {
	case score >= 7.5:
		label = "Rock Solid"
	case score >= 5.5:
		label = "Steady Option"
	case score >= 4.0:
		label = "Rotation Pick"
	default:
		label = ""
	}
	return
}
