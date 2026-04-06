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
	"gorm.io/gorm/clause"
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
		// Prevent bench/inactive players (0–2 matches) from scoring high due to
		// small-sample noise in per-90 components.
		if p.GamesPlayed < 3 {
			return p, false
		}
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
	// Compute all updated player rows in-memory, then batch persist.
	updates := make([]models.Player, len(players))

	for i := range players {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			p := players[i]
			updates[i] = s.enrichAndCompute(p, nextOpponentByTeam[p.TeamID])
		}(i)
	}

	wg.Wait()

	// Batch persist to drastically reduce "SLOW SQL" spam from per-player UPDATEs.
	if len(updates) > 0 {
		if err := s.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			UpdateAll: true,
		}).CreateInBatches(&updates, 200).Error; err != nil {
			fmt.Printf("[sync] Error during batch upsert: %v\n", err)
		}
	}

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

// enrichAndCompute fetches all stats for a player and computes the aggregate fields
// in-memory. It does not write to the DB; the caller can batch persist the result.
func (s *PredictionService) enrichAndCompute(p models.Player, nextOpponent string) models.Player {
	stats, err := s.client.GetPlayerStats(p.ID)
	if err != nil || len(stats) == 0 {
		return p
	}

	aggregateOverall(&p, stats)

	played := playedGames(stats)
	sortByDateDesc(played)
	if len(played) > 3 {
		played = played[:3]
	}

	p.RecentGamesPlayed = len(played)
	aggregateRecent(&p, played)

	p.NextOpponent = nextOpponent
	p.OpponentScore = playerVsOpponentScore(stats, nextOpponent)

	return p
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

func (s *PredictionService) GetPlayerPrediction(playerID uint) (*models.PlayerPrediction, error) {
	var player models.Player
	if err := s.db.First(&player, playerID).Error; err != nil {
		return nil, fmt.Errorf("player not found: %w", err)
	}
	stats, err := s.client.GetPlayerStats(playerID)
	if err == nil && len(stats) > 0 {
		aggregateOverall(&player, stats)
		s.db.Save(&player)
	}
	return s.calcPrediction(player), nil
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

func riskLevelFromPredictedScore(predictedScore float64) string {
	if predictedScore >= 7.0 {
		return "low"
	}
	if predictedScore >= 4.5 {
		return "medium"
	}
	return "high"
}

// pickRedFlagsByPositionQuota applies the same GK=1/DEF=2/MID=3/FWD=3 quota to
// a sorted (descending RedFlagScore) red-flag list and returns at most 9 players.
// This prevents one position from flooding the list when a tactical or seasonal
// trend affects an entire role (e.g., all strikers in poor form simultaneously).
func pickRedFlagsByPositionQuota(flags []models.RedFlagPlayer) []models.RedFlagPlayer {
	byPos := map[string][]models.RedFlagPlayer{
		"GK": {}, "DEF": {}, "MID": {}, "FWD": {},
	}
	for _, f := range flags {
		pos := canonicalPosition(f.Player.Position)
		byPos[pos] = append(byPos[pos], f)
	}
	// Each group is already sorted descending (caller sorts before invoking).
	result := make([]models.RedFlagPlayer, 0, 9)
	for pos, quota := range positionQuota {
		group := byPos[pos]
		if len(group) > quota {
			group = group[:quota]
		}
		result = append(result, group...)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].RedFlagScore > result[j].RedFlagScore
	})
	return result
}

// pickBenchwarmersByPositionQuota applies the same GK=1/DEF=2/MID=3/FWD=3 quota
// to a sorted (descending ConsistencyScore) benchwarmer list.
func pickBenchwarmersByPositionQuota(players []models.BenchwarmerPlayer) []models.BenchwarmerPlayer {
	byPos := map[string][]models.BenchwarmerPlayer{
		"GK": {}, "DEF": {}, "MID": {}, "FWD": {},
	}
	for _, bw := range players {
		pos := canonicalPosition(bw.Player.Position)
		byPos[pos] = append(byPos[pos], bw)
	}
	result := make([]models.BenchwarmerPlayer, 0, 9)
	for pos, quota := range positionQuota {
		group := byPos[pos]
		if len(group) > quota {
			group = group[:quota]
		}
		result = append(result, group...)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ConsistencyScore > result[j].ConsistencyScore
	})
	return result
}

func canonicalPosition(pos string) string {
	switch pos {
	case "GK", "DEF", "MID", "FWD":
		return pos
	default:
		return "FWD"
	}
}

// percentile returns a value at fraction frac (0..1) from a sorted ascending slice.
// Example: frac=0.05 => 5th percentile.
func percentile(sortedAsc []float64, frac float64) float64 {
	if len(sortedAsc) == 0 {
		return 0
	}
	if frac <= 0 {
		return sortedAsc[0]
	}
	if frac >= 1 {
		return sortedAsc[len(sortedAsc)-1]
	}
	// Use floor (not round) so percentiles close to 1.0 don't frequently select
	// the absolute max when the sample size is small.
	idx := int(math.Floor(frac * float64(len(sortedAsc)-1)))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sortedAsc) {
		idx = len(sortedAsc) - 1
	}
	return sortedAsc[idx]
}

func normalizeScoreTo0_10AllowTen(score, low, high float64, allowTen bool) float64 {
	if high <= low {
		return 5.0
	}
	n := (score - low) / (high - low) * 10.0
	if n < 0 {
		n = 0
	}
	// Clamp to the intended output range.
	if n > 10 {
		n = 10
	}
	// Keep full precision; the frontend formats to 1-2 decimals.
	return n
}

func normalizePlayerPredictedScoresByPosition(preds []models.PlayerPrediction) {
	byPos := map[string][]int{}
	for i := range preds {
		pos := canonicalPosition(preds[i].Player.Position)
		byPos[pos] = append(byPos[pos], i)
	}

	for pos := range byPos {
		indices := byPos[pos]
		scoresAsc := make([]float64, 0, len(indices))
		for _, idx := range indices {
			scoresAsc = append(scoresAsc, preds[idx].PredictedScore)
		}
		sort.Float64s(scoresAsc)

		// Use robust scaling so the top doesn't get flattened into lots of 10s.
		// Only the upper tail should approach 10.
		// Use robust tail bounds so only the very top tail approaches 10.
		low := percentile(scoresAsc, 0.005)
		high := percentile(scoresAsc, 0.995)

		// Sort descending so we know which player is "rank #1" inside this position group.
		sort.Slice(indices, func(a, b int) bool {
			return preds[indices[a]].PredictedScore > preds[indices[b]].PredictedScore
		})
		for rank, idx := range indices {
			allowTen := rank == 0
			norm := normalizeScoreTo0_10AllowTen(preds[idx].PredictedScore, low, high, allowTen)
			preds[idx].PredictedScore = norm
			preds[idx].RiskLevel = riskLevelFromPredictedScore(norm)
		}
	}
}

func normalizeRedFlagScoresByPosition(flags []models.RedFlagPlayer) {
	byPos := map[string][]int{}
	for i := range flags {
		pos := canonicalPosition(flags[i].Player.Position)
		byPos[pos] = append(byPos[pos], i)
	}

	for pos := range byPos {
		indices := byPos[pos]
		scoresAsc := make([]float64, 0, len(indices))
		for _, idx := range indices {
			scoresAsc = append(scoresAsc, flags[idx].RedFlagScore)
		}
		sort.Float64s(scoresAsc)

		// Use a slightly higher low tail to avoid extreme outliers,
		// but use the true max for the high bound so the top end doesn't
		// collapse into the hard cap.
		low := percentile(scoresAsc, 0.01)
		high := scoresAsc[len(scoresAsc)-1]

		// Sort descending so we know which player is "rank #1" inside this position group.
		sort.Slice(indices, func(a, b int) bool {
			return flags[indices[a]].RedFlagScore > flags[indices[b]].RedFlagScore
		})
		for rank, idx := range indices {
			allowTen := rank == 0
			flags[idx].RedFlagScore = normalizeScoreTo0_10AllowTen(flags[idx].RedFlagScore, low, high, allowTen)
		}
	}
}

func (s *PredictionService) GetTopPredictions(league, position, gemFilter, timeFilter string) ([]models.PlayerPrediction, error) {
	players, err := s.loadPlayers(league, position)
	if err != nil {
		return nil, err
	}
	preds := make([]models.PlayerPrediction, 0, len(players))
	for _, p := range players {
		scoring, ok := scoringView(p, timeFilter)
		if !ok {
			continue
		}
		pred := s.calcPrediction(scoring)
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

	// When "All Positions" are requested, we normalize for ranking fairness
	// but we MUST return the raw predicted score so it matches the player
	// profile endpoint (/api/predict/player/{id}), which uses calcPrediction.
	if position == "" {
		type rawInfo struct {
			score float64
			risk  string
		}
		rawByID := make(map[uint]rawInfo, len(preds))
		for _, pr := range preds {
			rawByID[pr.Player.ID] = rawInfo{score: pr.PredictedScore, risk: pr.RiskLevel}
		}

		// Copy for ordering only.
		ordering := make([]models.PlayerPrediction, len(preds))
		copy(ordering, preds)
		normalizePlayerPredictedScoresByPosition(ordering)

		// Apply position quota (GK=1 DEF=2 MID=3 FWD=3) using normalized scores so
		// each position competes fairly within itself before the cross-position cut.
		// This is what prevents one position from flooding the top-9 list.
		ordering = pickTopWithPositionQuota(ordering)

		// Restore raw score + raw risk level for UI consistency.
		for i := range ordering {
			if raw, ok := rawByID[ordering[i].Player.ID]; ok {
				ordering[i].PredictedScore = raw.score
				ordering[i].RiskLevel = raw.risk
			}
		}

		// Order the returned list by the same score we display in the UI.
		// Without this, the list order is based on position-normalized score
		// while the displayed predicted_score is raw, which can look "unsorted".
		sort.Slice(ordering, func(i, j int) bool {
			return ordering[i].PredictedScore > ordering[j].PredictedScore
		})

		return ordering, nil
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
		// Keep the "reasons" gating always (empty reasons = not a meaningful red flag),
		// but defer the numeric ">= 4.0" threshold when showing all positions so
		// position groups are comparable.
		if len(reasons) == 0 {
			continue
		}
		if position != "" && score < 4.0 {
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

	// Cross-position fairness: normalize by position when "all positions" are requested.
	if position == "" {
		// Keep the raw RedFlagScore (computed by calcRedFlag) so the UI reflects
		// the actual underlying decline severity.
		filtered := make([]models.RedFlagPlayer, 0, len(result))
		for _, rf := range result {
			if rf.RedFlagScore >= 4.0 {
				filtered = append(filtered, rf)
			}
		}
		result = filtered
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].RedFlagScore > result[j].RedFlagScore
	})
	if position == "" {
		result = pickRedFlagsByPositionQuota(result)
	} else if len(result) > 9 {
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

	result := make([]models.BenchwarmerPlayer, 0)

	if position == "" {
		// Keep raw ConsistencyScore so it doesn't get renormalized into a 0–10
		// per-position scale (which can make multiple players land on the cap).
		for _, p := range players {
			scoring, ok := scoringView(p, timeFilter)
			if !ok {
				continue
			}

			pred := s.calcPrediction(scoring)
			if pred.HiddenGem {
				continue
			}

			// Exclude players already showing meaningful red-flag reasons.
			rfScore, _, _, reasons := calcRedFlag(p)
			if rfScore >= 4.0 && len(reasons) > 0 {
				continue
			}

			consistency, label := calcBenchwarmer(scoring)
			if label == "" {
				continue
			}

			result = append(result, models.BenchwarmerPlayer{
				Player:           scoring,
				ConsistencyScore: consistency,
				Label:            label,
			})
		}
	} else {
		// Position-specific view: keep the existing absolute thresholds.
		for _, p := range players {
			scoring, ok := scoringView(p, timeFilter)
			if !ok {
				continue
			}
			pred := s.calcPrediction(scoring)
			if pred.HiddenGem {
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
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ConsistencyScore > result[j].ConsistencyScore
	})
	if position == "" {
		result = pickBenchwarmersByPositionQuota(result)
	} else if len(result) > 9 {
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

func (s *PredictionService) GetSynergy(playerIDs []uint) (*models.SynergyResult, error) {
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
		total += s.calcPrediction(p).PredictedScore
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

// formComponent blends season and recent match ratings for a forward-looking form signal.
// Recent form is weighted more heavily since it better predicts the upcoming match.
// GKs get a 50/50 blend because a string of recent clean sheets (or blunders) matters more
// game-to-game for a keeper than for outfield players.
func formComponent(p models.Player) float64 {
	seasonForm := p.FormScore
	if seasonForm <= 0 {
		seasonForm = 6.0
	}
	if p.RecentFormScore > 0 && p.RecentGamesPlayed >= 2 {
		if p.Position == "GK" {
			// GK: 50/50 — recent clean sheets / howlers are strongly predictive
			return math.Max(0, math.Min(10, seasonForm*0.50+p.RecentFormScore*0.50))
		}
		// Outfield: 60% season stability + 40% recent form
		return math.Max(0, math.Min(10, seasonForm*0.60+p.RecentFormScore*0.40))
	}
	return math.Max(0, math.Min(10, seasonForm))
}

// attackComponent measures goal-scoring threat with three independent signals.
// Recent stats are blended in (40% weight) to capture current offensive momentum.
//
// xG scaling is position-aware so the cap reflects realistic xG ranges per role:
//   - FWD: cap 10 (multiplier ×14) — elite strikers (0.70+ xG/90) reach the cap;
//     0.40→5.6, 0.57→8.0, 0.71→10.0. Haaland and a squad striker are now distinct.
//   - MID: cap 8  (multiplier ×14) — 0.57 xG/90 is exceptional for a MID.
//   - DEF: cap 5  (multiplier ×10) — DEF goals are rare; even 0.50 xG/90 is elite.
//   - GK:  cap 3  (multiplier ×6)  — attack contribution is negligible; weight=0 in calcPrediction.
//
// Conversion bonus (+0-2): rewards clinical finishers without double-counting xG.
// Shot volume (+0-2):      how often the player tests the keeper regardless of xG quality.
func attackComponent(p models.Player) float64 {
	mins90 := math.Max(1, float64(p.MinutesPlayed)/90.0)
	xgPer90 := p.XG / mins90
	goalsPer90 := float64(p.Goals) / mins90
	sotPer90 := float64(p.ShotsOnTarget) / mins90

	// Blend in recent stats when available — captures current offensive momentum
	if p.RecentGamesPlayed >= 2 && p.RecentMinutes > 0 {
		recentMins90 := math.Max(1, float64(p.RecentMinutes)/90.0)
		xgPer90 = xgPer90*0.60 + (p.RecentXG/recentMins90)*0.40
		goalsPer90 = goalsPer90*0.60 + (float64(p.RecentGoals)/recentMins90)*0.40
		sotPer90 = sotPer90*0.60 + (float64(p.RecentShotsOnTarget)/recentMins90)*0.40
	}

	// Position-specific xG ceiling — prevents FWDs from plateauing at the same
	// score as midfielders just because the old cap was set for outfield averages.
	var xgMult, xgCap float64
	switch p.Position {
	case "FWD":
		xgMult, xgCap = 14.0, 10.0 // 0.57→8.0, 0.71→10 (Haaland range)
	case "DEF":
		xgMult, xgCap = 10.0, 5.0 // DEF scoring is rare; cap reflects that
	case "GK":
		xgMult, xgCap = 6.0, 3.0 // practically irrelevant (weight 0 in formula)
	default: // MID
		xgMult, xgCap = 14.0, 8.0 // original scaling
	}

	xgScore := math.Min(xgCap, xgPer90*xgMult)
	conversionBonus := math.Min(2, math.Max(0, goalsPer90-xgPer90)*4)
	shotVolume := math.Min(2, sotPer90*0.6)

	return math.Min(10, xgScore+conversionBonus+shotVolume)
}

// creativityComponent is fully position-specific — each position uses the stats
// that are genuinely meaningful for that role.
//
// DEF (build-up quality, not chance creation):
//   xA/assists are near-zero for most defenders, so using them as the primary
//   signal collapses all DEFs into the same 0-2 range. Instead:
//   - Key passes per 90 (0-8): 0.5/90→3, 1.0/90→6, 1.5/90→9 — elite attacking FB.
//   - Assist delivery bonus (0-2): DEFs whose key passes actually produce assists
//     confirm that their crosses and through balls are genuinely dangerous.
//   - Recent blend (60/40): captures attacking FB hot/cold streaks.
//
// MID / FWD (chance creation):
//   1. xA per 90 — quality of chances created (primary predictive signal).
//      Blended 60% season / 40% recent. Elite playmaker ≈ 0.35 xA/90 → ~5.6.
//   2. Assist delivery bonus — assists/90 above xA/90 (team-mates converting well).
//   3. Key pass quality — xA per key pass (dangerous passes, not speculative ones).
//   4. Pass accuracy (MID only, 0-2): 72%→0, 80%→0.9, 88%→2.0.
//      Differentiates tidy ball-players from ball-losers in the build-up.
func creativityComponent(p models.Player) float64 {
	mins90 := math.Max(1, float64(p.MinutesPlayed)/90.0)

	// ── DEF: build-up quality ────────────────────────────────────────────────
	// Attacking full-backs (1.5 KP/90) score ~9; typical CBs (0.3 KP/90) score ~2.
	// This gives the creativity weight real range across the DEF pool, so ball-playing
	// CBs and attacking FBs are correctly differentiated from limited defenders.
	if p.Position == "DEF" {
		kpPer90 := float64(p.KeyPasses) / mins90
		if p.RecentGamesPlayed >= 2 && p.RecentMinutes > 0 {
			recentMins90 := math.Max(1, float64(p.RecentMinutes)/90.0)
			kpPer90 = kpPer90*0.60 + (float64(p.RecentKeyPasses)/recentMins90)*0.40
		}
		kpScore := math.Min(8, kpPer90*6) // 1.33 KP/90 → 8.0 (attacking FB ceiling)
		assistBonus := 0.0
		if p.KeyPasses > 0 {
			// Rewards DEFs whose key passes actually convert to assists
			assistBonus = math.Min(2, float64(p.Assists)/float64(p.KeyPasses)*8)
		}
		return math.Min(10, kpScore+assistBonus)
	}

	// ── MID / FWD: chance creation ───────────────────────────────────────────
	xaPer90 := p.XA / mins90
	assistsPer90 := float64(p.Assists) / mins90

	if p.RecentGamesPlayed >= 2 && p.RecentMinutes > 0 {
		recentMins90 := math.Max(1, float64(p.RecentMinutes)/90.0)
		xaPer90 = xaPer90*0.60 + (p.RecentXA/recentMins90)*0.40
		assistsPer90 = assistsPer90*0.60 + (float64(p.RecentAssists)/recentMins90)*0.40
	}

	xaScore := math.Min(8, xaPer90*16)
	assistBonus := math.Min(2, math.Max(0, assistsPer90-xaPer90)*4)
	kpQuality := 0.0
	if p.KeyPasses > 0 {
		xaPerKP := p.XA / float64(p.KeyPasses)
		kpQuality = math.Min(2, xaPerKP*20) // 0.10 xA/KP → 2.0
	}

	// Pass accuracy bonus — MID only; season totals used for statistical stability.
	// 72%→0, 80%→0.9, 88%→2.0. Differentiates tidy ball-players from ball-losers.
	passAccBonus := 0.0
	if p.Position == "MID" && p.TotalPasses >= 15 {
		passAcc := float64(p.AccuratePasses) / float64(p.TotalPasses)
		passAccBonus = math.Max(0, math.Min(2, (passAcc-0.72)/0.18*2))
	}

	return math.Min(10, xaScore+assistBonus+kpQuality+passAccBonus)
}

// defensiveComponent is fully position-specific.
//
//   - GK:  four signals — save rate (0-7), goals-conceded rate (0-2), recent trend
//     (±1.5), and save volume (0-0.5). The four-signal approach produces a much wider
//     spread than save rate alone, which clusters most keepers in a narrow 65-80%
//     band. Example spread: elite GK (~85% SR, 0.7 GC/g) → ~9-10; average (~72%, 1.2
//     GC/g) → ~6-7; poor (~60%, 1.8 GC/g) → ~2-3.
//     Minimum threshold raised to 2 games and 5 shots faced to avoid single-game noise.
//
//   - DEF: duel win rate (0-4.5) + tackle win rate (0-4.0) + activity volume (0-1.5)
//     + pass accuracy bonus (0-1.5) for ball-playing defenders. The pass accuracy
//     signal differentiates modern ball-playing CBs and attacking full-backs from
//     more limited defenders who can only defend.
//
//   - MID: duel win rate (0-5) + tackle win rate (0-3) + floor of 1.0. Two signals
//     instead of one gives much better spread: a press-heavy DM who wins 65% of
//     duels and 70% of tackles scores ≈8; a creative AM avoiding duels scores ≈4.5.
//
//   - FWD: hold-up play (duel win rate only). Score range 1–6.
func defensiveComponent(p models.Player) float64 {
	mins90 := math.Max(1, float64(p.MinutesPlayed)/90.0)

	switch p.Position {
	case "GK":
		games := math.Max(1, float64(p.GamesPlayed))
		total := float64(p.Saves + p.GoalsConceded)
		// Need at least 2 games and 5 shots faced to have reliable save rate data
		if p.GamesPlayed < 2 || total < 5 {
			return 6.0 // insufficient data — neutral
		}
		saveRate := float64(p.Saves) / total

		// Signal 1: Save rate (0-7) — primary quality signal.
		// 50%→0, 65%→3.5, 72%→5.1, 80%→6.7, 87%→7 (elite).
		// Steeper curve than before to better separate keepers in the 65-82% range.
		rateScore := math.Max(0, math.Min(7, (saveRate-0.50)/0.37*7))

		// Signal 2: Goals conceded per game (0-2) — explicit conceding penalty.
		// Elite (<0.7/game)→2.0; average (1.2/game)→1.1; poor (≥1.8/game)→0.
		goalsConcededPerGame := float64(p.GoalsConceded) / games
		gcScore := math.Max(0, math.Min(2, (1.8-goalsConcededPerGame)/1.1*2))

		// Signal 3: Recent save-rate trend (−1.5 to +1.5).
		// Rewards GKs improving their save rate over the last 3 games; penalises
		// those in poor form. Each 5% swing in save rate = ±0.5.
		trendBonus := 0.0
		recentTotal := float64(p.RecentSaves + p.RecentGoalsConceded)
		if p.RecentGamesPlayed >= 2 && recentTotal >= 3 {
			recentSaveRate := float64(p.RecentSaves) / recentTotal
			trendBonus = math.Max(-1.5, math.Min(1.5, (recentSaveRate-saveRate)*10))
		}

		// Signal 4: Save volume per game (0-0.5) — small bonus for busy, reliable GKs.
		// 3 saves/game → +0.25; 6+ saves/game → +0.5.
		savesPerGame := float64(p.Saves) / games
		volumeBonus := math.Min(0.5, savesPerGame/6.0*0.5)

		// Signal 5: Pass accuracy (0-1.0) — sweeper keeper / ball-playing GK quality.
		// Modern keepers are expected to play out from the back under pressure.
		// Poor distributors are a liability in high-press systems.
		// Requires ≥10 passes logged for statistical reliability.
		// 55%→0, 70%→0.6, 80%+→1.0.
		gkPassAcc := 0.0
		if p.TotalPasses >= 10 {
			passAcc := float64(p.AccuratePasses) / float64(p.TotalPasses)
			gkPassAcc = math.Max(0, math.Min(1.0, (passAcc-0.55)/0.25))
		}

		return math.Max(0, math.Min(10, rateScore+gcScore+trendBonus+volumeBonus+gkPassAcc))

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
		tacklesPer90 := float64(p.TacklesTotal) / mins90
		activityBonus := math.Min(1.5, tacklesPer90/4.0*1.5)

		// Pass accuracy bonus (0-1.5): ball-playing DEFs who complete passes accurately
		// score higher. Target: 65%→0, 78%→0.78, 90%→1.5.
		// Uses season total for sample stability; requires at least 15 passes logged.
		passAccBonus := 0.0
		if p.TotalPasses >= 15 {
			passAcc := float64(p.AccuratePasses) / float64(p.TotalPasses)
			passAccBonus = math.Max(0, math.Min(1.5, (passAcc-0.65)/0.25*1.5))
		}

		// Quality rates (0-4.5 duel, 0-4.0 tackle) + volume + pass accuracy
		return math.Min(10, duelRate*4.5+tackleRate*4.0+activityBonus+passAccBonus)

	case "MID":
		duelRate := 0.50
		if p.DuelsTotal > 0 {
			duelRate = float64(p.DuelsWon) / float64(p.DuelsTotal)
		}
		tackleRate := 0.50
		if p.TacklesTotal > 0 {
			tackleRate = float64(p.TacklesWon) / float64(p.TacklesTotal)
		}
		// Two signals instead of one: presses + ball-winners get separate credit.
		// DM winning 65% duels + 70% tackles → ~8.1; AM avoiding duels → ~4.5 floor.
		return math.Min(10, duelRate*5.0+tackleRate*3.0+1.0)

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
//
// Blend is 50% season / 50% recent. The previous 70/30 split was too slow to
// detect rotation: a player averaging 88 min/game all season who has been
// subbed at 45 min for 3 straight games still scored ~8.6 instead of signalling
// the rotation risk. At 50/50 that same pattern scores ~7.2 — a meaningful drop.
func availabilityComponent(p models.Player) float64 {
	gamesSeason := math.Max(1, float64(p.GamesPlayed))
	avgMinsSeason := float64(p.MinutesPlayed) / gamesSeason
	availSeason := avgMinsSeason / 90.0 * 10.0

	recentGames := math.Max(1, float64(p.RecentGamesPlayed))
	avgMinsRecent := float64(p.RecentMinutes) / recentGames
	availRecent := avgMinsRecent / 90.0 * 10.0

	avail := availSeason*0.50 + availRecent*0.50
	if avail < 0 {
		avail = 0
	}
	return math.Min(10, avail)
}

// disciplineComponent penalises cards. Starting at 10:
// each yellow card per game costs 5 points; each red costs 15.
//
// Blend is 40% season / 60% recent. Suspension risk is primarily driven by
// recent behaviour — a player picking up 3 yellows in 3 games is imminently
// suspended regardless of how clean they were earlier in the season. The old
// 70/30 blend buried that signal under a clean season record.
func disciplineComponent(p models.Player) float64 {
	gamesSeason := math.Max(1, float64(p.GamesPlayed))
	yellowPerGameSeason := float64(p.YellowCards) / gamesSeason
	redPerGameSeason := float64(p.RedCards) / gamesSeason

	recentGames := math.Max(1, float64(p.RecentGamesPlayed))
	yellowPerGameRecent := float64(p.RecentYellowCards) / recentGames
	redPerGameRecent := float64(p.RecentRedCards) / recentGames

	// 40% season base rate + 60% recent — recent cards dominate (suspension risk)
	yellowPerGame := yellowPerGameSeason*0.40 + yellowPerGameRecent*0.60
	redPerGame := redPerGameSeason*0.40 + redPerGameRecent*0.60

	disc := 10 - yellowPerGame*5 - redPerGame*15
	if disc < 0 {
		disc = 0
	}
	return disc
}

// ─── Prediction ───────────────────────────────────────────────────────────────

// calcPrediction combines seven independent components with position-specific weights.
//
// Each component is now enriched with position-specific signals and recent-form blending
// (see individual component functions for details). Weight rationale per position:
//
//	GK:  Defensive dominates (0.60). The defensive component now has 4 signals
//	     (save rate, goals-conceded rate, recent trend, volume) producing a much
//	     wider score spread than before. Form uses a 50/50 season/recent blend.
//	     Availability and discipline stay low — GKs rarely miss games or get carded.
//	     Weights: form=0.22, def=0.60, avail=0.06, disc=0.04, opp=0.08  → sum 1.00
//
//	DEF: Defensive work is primary (0.30). Now includes pass accuracy signal so
//	     ball-playing CBs and attacking full-backs score distinctly higher.
//	     Form uses a 60/40 season/recent blend; attack/creativity use same blend.
//	     Weights: form=0.22, atk=0.08, cre=0.07, def=0.30, avail=0.13, disc=0.10, opp=0.10 → 1.00
//
//	MID: Most balanced role. Creativity includes pass accuracy bonus (tidy MIDs
//	     score higher). Defensive component now uses two signals (duel + tackle rate)
//	     instead of one, giving better spread between DMs and AMs.
//	     Weights: form=0.20, atk=0.17, cre=0.24, def=0.12, avail=0.10, disc=0.09, opp=0.08 → 1.00
//
//	FWD: Attack dominates (0.33) with recent xG/goals blended in.
//	     Opponent weight raised to 0.12 — strikers are the most affected by defensive
//	     quality. Defensive contribution minimal (0.02): hold-up play only.
//	     Weights: form=0.18, atk=0.33, cre=0.17, def=0.02, avail=0.10, disc=0.08, opp=0.12 → 1.00
func (s *PredictionService) calcPrediction(player models.Player) *models.PlayerPrediction {
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

	// ── Sub-role inference ────────────────────────────────────────────────────
	// We only have four positions (GK/DEF/MID/FWD) but real footballers play very
	// different roles within each. We infer the sub-role from the actual stat
	// signature and adjust weights accordingly. This prevents a DM being penalised
	// by the creativity-heavy weights designed for an AM, and stops a CB being
	// unfairly compared against attacking full-backs on the same weight set.
	gamesF := math.Max(1, float64(player.GamesPlayed))
	mins90F := math.Max(1, float64(player.MinutesPlayed)/90.0)

	// DEF sub-role: attacking full-back vs center-back.
	// Attacking FB: ≥1.0 key passes per game OR ≥0.15 assists per game.
	// CB: primarily defensive, creativity contribution is minimal.
	defKPperGame := float64(player.KeyPasses) / gamesF
	defAssistsPerGame := float64(player.Assists) / gamesF
	isAttackingFB := defKPperGame >= 1.0 || defAssistsPerGame >= 0.15

	// MID sub-role: holding/defensive mid vs attacking/box-to-box mid.
	// DM: high duel volume per 90 (≥6) AND low key pass output (<1.5/game).
	// AM/box-to-box: creativity is primary, defensive work is secondary.
	midDuelsPer90 := float64(player.DuelsTotal) / mins90F
	midKPperGame := float64(player.KeyPasses) / gamesF
	isDM := midDuelsPer90 >= 6.0 && midKPperGame < 1.5

	// FWD sub-role: second striker / false nine vs pure striker.
	// Creative FWD: creativity score ≥5.5 OR ≥0.20 assists per game.
	// Pure striker: attack-first, creativity is supplementary.
	fwdAssistsPerGame := float64(player.Assists) / gamesF
	isCreativeFWD := creativity >= 5.5 || fwdAssistsPerGame >= 0.20

	var predicted float64
	var numerator float64
	var denom float64
	switch player.Position {
	case "GK":
		// Defensive dominates (0.60). Pass accuracy now in the defensive component
		// (Signal 5) so GK distribution quality is already captured there.
		numerator = form*0.22 +
			defensive*0.60 +
			availability*0.06 +
			discipline*0.04 +
			opponent*0.08
		denom = 0.22 + 0.60 + 0.06 + 0.04 + 0.08

	case "DEF":
		if isAttackingFB {
			// Attacking full-back: creativity (key passes / crosses) is a real value
			// driver alongside defensive work. Attack weight also rises slightly to
			// capture set-piece contributions and occasional goals.
			// form=0.20 atk=0.10 cre=0.14 def=0.28 avail=0.12 disc=0.08 opp=0.08 → 1.00
			numerator = form*0.20 +
				attack*0.10 +
				creativity*0.14 +
				defensive*0.28 +
				availability*0.12 +
				discipline*0.08 +
				opponent*0.08
			denom = 0.20 + 0.10 + 0.14 + 0.28 + 0.12 + 0.08 + 0.08
		} else {
			// Center-back: defensive dominates; creativity and attack near-zero weights
			// reflect that CB value is almost entirely defensive.
			// form=0.22 atk=0.05 cre=0.05 def=0.35 avail=0.13 disc=0.10 opp=0.10 → 1.00
			numerator = form*0.22 +
				attack*0.05 +
				creativity*0.05 +
				defensive*0.35 +
				availability*0.13 +
				discipline*0.10 +
				opponent*0.10
			denom = 0.22 + 0.05 + 0.05 + 0.35 + 0.13 + 0.10 + 0.10
		}

	case "MID":
		if isDM {
			// Defensive midfielder: ball-winning and press coverage is primary.
			// Defensive weight almost doubles vs the AM weights; creativity drops.
			// form=0.20 atk=0.10 cre=0.14 def=0.24 avail=0.11 disc=0.11 opp=0.10 → 1.00
			numerator = form*0.20 +
				attack*0.10 +
				creativity*0.14 +
				defensive*0.24 +
				availability*0.11 +
				discipline*0.11 +
				opponent*0.10
			denom = 0.20 + 0.10 + 0.14 + 0.24 + 0.11 + 0.11 + 0.10
		} else {
			// Attacking / box-to-box mid: creativity is primary.
			// form=0.20 atk=0.17 cre=0.24 def=0.12 avail=0.10 disc=0.09 opp=0.08 → 1.00
			numerator = form*0.20 +
				attack*0.17 +
				creativity*0.24 +
				defensive*0.12 +
				availability*0.10 +
				discipline*0.09 +
				opponent*0.08
			denom = 0.20 + 0.17 + 0.24 + 0.12 + 0.10 + 0.09 + 0.08
		}

	default: // FWD
		if isCreativeFWD {
			// Second striker / false nine: creativity and attack weighted more equally.
			// form=0.18 atk=0.27 cre=0.23 def=0.02 avail=0.10 disc=0.08 opp=0.12 → 1.00
			numerator = form*0.18 +
				attack*0.27 +
				creativity*0.23 +
				defensive*0.02 +
				availability*0.10 +
				discipline*0.08 +
				opponent*0.12
			denom = 0.18 + 0.27 + 0.23 + 0.02 + 0.10 + 0.08 + 0.12
		} else {
			// Pure striker: attack dominates; creativity supplementary.
			// form=0.18 atk=0.35 cre=0.14 def=0.02 avail=0.10 disc=0.08 opp=0.13 → 1.00
			numerator = form*0.18 +
				attack*0.35 +
				creativity*0.14 +
				defensive*0.02 +
				availability*0.10 +
				discipline*0.08 +
				opponent*0.13
			denom = 0.18 + 0.35 + 0.14 + 0.02 + 0.10 + 0.08 + 0.13
		}
	}
	if denom <= 0 {
		predicted = 6.0
	} else {
		predicted = numerator / denom
	}

	// Sample-size confidence dampening — pulls the score toward the position-neutral
	// (5.5) for players with few games. Without this, a player who scored a hat-trick
	// across their first 3 games would rank above established regulars purely because
	// 3-game per-90 stats are extreme.
	//
	// Confidence rises linearly from 0.55 at 3 games to 1.00 at 20 games.
	// In "recent" mode all players have GamesPlayed=3 (equal dampening → no ranking
	// effect in recent mode, but raw scores are slightly stabilised).
	//
	//   3 games  → 55% actual + 45% neutral(5.5)
	//   10 games → 87% actual + 13% neutral
	//   20+ games→ 100% actual (no dampening)
	const neutralScore = 5.5
	confidence := math.Min(1.0, 0.55+float64(player.GamesPlayed-3)*(0.45/17.0))
	predicted = predicted*confidence + neutralScore*(1.0-confidence)

	// Keep more precision to reduce artificial ties after normalization.
	predicted = math.Round(predicted*1000) / 1000

	risk := "high"
	if predicted >= 7.0 {
		risk = "low"
	} else if predicted >= 4.5 {
		risk = "medium"
	}

	hiddenGem, gemReasons := isHiddenGem(player, predicted, attack, creativity)

	return &models.PlayerPrediction{
		Player:             player,
		PredictedScore:     predicted,
		RiskLevel:          risk,
		HiddenGem:          hiddenGem,
		HiddenGemReasons:  gemReasons,
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
// AND fewer than a position-typical G+A/goal output (not yet well-known or expensively priced in).
//
// Seven independent signals — any single one qualifies the player:
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
//  4. Strong attack component but low returns for the player's position:
//     quality threat not yet visible in the stat line (new team, early season etc).
//
//  5. High xG per shot (≥0.12): the player takes shots from premium positions
//     (inside the box, one-on-ones) but hasn't scored yet. Quality > quantity.
//
//  6. Improving trajectory: recent xG+xA/90 is ≥30% above the season average,
//     meaning underlying form is genuinely trending up right now.
//
//  7. Position-specific specialist quality (see inline comments per position).
func isHiddenGem(p models.Player, predicted, attackScore, creativityScore float64) (bool, []string) {
	if predicted < 4.5 || predicted >= 8.0 {
		return false, nil
	}

	// Position-aware thresholds:
	// - defenders naturally have fewer goals/assists than mids/forwards
	// - forwards can still be "hidden gems" with slightly higher totals
	//   if they are underperforming their underlying xG/xA signals.
	maxGATotal := 12
	lowReturns := 6
	lowGoals := 3
	switch p.Position {
	case "GK":
		maxGATotal = 6
		lowReturns = 1
		lowGoals = 1
	case "DEF":
		maxGATotal = 10
		lowReturns = 4
		lowGoals = 2
	case "MID":
		maxGATotal = 12
		lowReturns = 6
		lowGoals = 3
	default: // FWD
		maxGATotal = 14
		lowReturns = 8
		lowGoals = 5
	}

	if p.Goals+p.Assists >= maxGATotal {
		return false, nil // already well-known / priced in
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
	highThreatLowReturns := attackScore >= 5.0 && p.Goals+p.Assists < lowReturns

	// Signal 5: taking high-quality shots but not converting yet
	highQualityPositions := false
	if p.TotalShots >= 6 {
		xgPerShot := p.XG / float64(p.TotalShots)
		highQualityPositions = xgPerShot >= 0.12 && p.Goals < lowGoals
	}

	// Signal 6: recent underlying stats trending clearly upward
	improvingTrajectory := false
	if p.RecentGamesPlayed >= 3 && p.RecentMinutes > 0 && xgXaPer90 > 0.05 {
		recentXT90 := (p.RecentXG + p.RecentXA) / math.Max(1, float64(p.RecentMinutes)/90.0)
		improvingTrajectory = recentXT90 > xgXaPer90*1.30
	}

	// Signal 7: position-specific specialist quality that standard signals miss.
	//
	//  GK:  Underrated keeper on a struggling team. High saves/game + high goals
	//       conceded/game + good save rate = GK bailing out a poor defence.
	//       Strong upside if the team improves or the keeper moves.
	//
	//  DEF: Ball-playing defender. Elite pass accuracy (≥87%) with solid duel win
	//       rate signals a modern CB/FB whose build-up value isn't visible in G+A.
	//
	//  MID: Box-to-box engine. High pressing volume (≥5 duels/90) combined with
	//       meaningful creativity flags an all-action midfielder whose work rate
	//       and range are undervalued by casual G+A-based assessment.
	//
	//  FWD: Pure poacher. xG per shot ≥0.16 means the player consistently occupies
	//       premium positions (penalty area, one-on-ones) — a higher bar than signal 5.
	positionGem := false
	positionGemReason := ""
	switch p.Position {
	case "GK":
		if p.GamesPlayed >= 5 && p.Saves > 0 {
			savesPerGame := float64(p.Saves) / float64(p.GamesPlayed)
			gcPerGame := float64(p.GoalsConceded) / float64(p.GamesPlayed)
			total := float64(p.Saves + p.GoalsConceded)
			saveRate := float64(p.Saves) / total
			if savesPerGame >= 3.0 && gcPerGame >= 1.2 && saveRate >= 0.68 {
				positionGem = true
				positionGemReason = "Overperforming behind a weak defence"
			}
		}
	case "DEF":
		if p.TotalPasses >= 30 && p.GamesPlayed >= 5 {
			passAcc := float64(p.AccuratePasses) / float64(p.TotalPasses)
			duelRate := 0.5
			if p.DuelsTotal > 0 {
				duelRate = float64(p.DuelsWon) / float64(p.DuelsTotal)
			}
			if passAcc >= 0.87 && duelRate >= 0.52 && p.Goals+p.Assists < 5 {
				positionGem = true
				positionGemReason = "Ball-playing defender with elite distribution"
			}
		}
	case "MID":
		if p.GamesPlayed >= 5 {
			duelsPer90 := float64(p.DuelsTotal) / mins90
			if duelsPer90 >= 5.0 && creativityScore >= 4.5 && p.Goals+p.Assists < 8 {
				positionGem = true
				positionGemReason = "High-energy engine with creative upside"
			}
		}
	default: // FWD
		if p.TotalShots >= 10 {
			xgPerShot := p.XG / float64(p.TotalShots)
			if xgPerShot >= 0.16 && p.Goals < lowGoals {
				positionGem = true
				positionGemReason = "Premium shooting positions, conversion incoming"
			}
		}
	}

	reasons := make([]string, 0, 4)
	if underperformingExpected {
		reasons = append(reasons, "Expected threat is real (xG+xA > G+A)")
	}
	if prolificShooter {
		reasons = append(reasons, "High chances, low conversion")
	}
	if creativeButUnrewarded {
		reasons = append(reasons, "Creating well, assists lag")
	}
	if highThreatLowReturns {
		reasons = append(reasons, "Strong threat, few returns")
	}
	if highQualityPositions {
		reasons = append(reasons, "Quality shots, not finishing yet")
	}
	if improvingTrajectory {
		reasons = append(reasons, "Underlying trend is improving")
	}
	if positionGem {
		reasons = append(reasons, positionGemReason)
	}

	if len(reasons) == 0 {
		return false, nil
	}
	return true, reasons
}

// ─── Red flags ────────────────────────────────────────────────────────────────

// calcRedFlag always receives the full player (both overall and recent stats intact)
// so it can detect true decline rather than just absolute badness.
//
// Ten signals are computed and combined with position-specific weights.
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

	// ── 8. DEF: duel win rate decline (losing ground in physical battles) ────
	// Compares recent duel win rate against the season average.
	// A defender who was winning 58% of duels all season but is now at 42% is a
	// concrete defensive red flag — opponents are increasingly getting past them.
	duelWinDecline := 0.0
	if p.Position == "DEF" && p.DuelsTotal >= 15 && p.RecentDuelsTotal >= 4 {
		overallDuelRate := float64(p.DuelsWon) / float64(p.DuelsTotal)
		recentDuelRate := float64(p.RecentDuelsWon) / float64(p.RecentDuelsTotal)
		if overallDuelRate > 0.45 {
			duelWinDecline = math.Max(0, (overallDuelRate-recentDuelRate)/overallDuelRate*10)
		}
	}

	// ── 9. MID: key pass contribution decline (creative influence fading) ─────
	// A midfielder whose key passes per 90 have dropped sharply is losing their
	// creative impact — often an early sign of fatigue, loss of role, or confidence.
	keyPassDecline := 0.0
	if p.Position == "MID" && p.KeyPasses >= 8 && p.RecentMinutes > 0 {
		overallKP90 := float64(p.KeyPasses) / mins90
		recentKP90 := float64(p.RecentKeyPasses) / recentMins90
		if overallKP90 > 0.4 {
			keyPassDecline = math.Max(0, (overallKP90-recentKP90)/overallKP90*10)
		}
	}

	// ── 10. FWD: xG per shot decline (moving to worse shooting positions) ─────
	// A striker whose xG per shot is falling is no longer getting into premium
	// positions — suggesting defensive marking, loss of runs, or tactical demotion.
	xGPerShotDecline := 0.0
	if p.Position == "FWD" && p.TotalShots >= 10 && p.RecentTotalShots >= 3 {
		overallXGperShot := p.XG / float64(p.TotalShots)
		recentXGperShot := p.RecentXG / float64(p.RecentTotalShots)
		if overallXGperShot > 0.08 {
			xGPerShotDecline = math.Max(0, (overallXGperShot-recentXGperShot)/overallXGperShot*10)
		}
	}

	// ── Composite (position-weighted) ────────────────────────────────────────
	switch p.Position {
	case "GK":
		score = formDecline*0.25 + gkDecline*0.45 + disciplineRisk*0.10 + involvementDecline*0.20
	case "DEF":
		// duelWinDecline replaces some of xThreatDecline weight — defenders rarely
		// generate xT themselves, but losing duels is a direct defensive red flag.
		score = formDecline*0.22 + outputDrop*0.15 + xThreatDecline*0.10 +
			involvementDecline*0.18 + disciplineRisk*0.20 + duelWinDecline*0.15
	case "MID":
		// keyPassDecline gets its own slice because midfield creativity is a primary
		// value driver — fading key passes often precede a full output decline.
		score = formDecline*0.22 + outputDrop*0.18 + xThreatDecline*0.14 +
			shotAccDecline*0.08 + involvementDecline*0.14 + disciplineRisk*0.10 + keyPassDecline*0.14
	default: // FWD
		// xGPerShotDecline replaces some outputDrop weight — for forwards, declining
		// shot quality is an earlier and more predictive signal than raw G+A drop.
		score = formDecline*0.21 + outputDrop*0.24 + xThreatDecline*0.17 +
			shotAccDecline*0.12 + involvementDecline*0.07 + disciplineRisk*0.04 + xGPerShotDecline*0.15
	}
	// Keep more precision to reduce artificial ties after normalization.
	score = math.Round(math.Min(10, score)*1000) / 1000

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
	if p.Position == "DEF" && duelWinDecline >= 5 {
		reasons = append(reasons, "Losing more duels recently — defensive reliability dropping")
	}
	if p.Position == "MID" && keyPassDecline >= 5 {
		reasons = append(reasons, "Creative output fading — fewer key passes in recent games")
	}
	if p.Position == "FWD" && xGPerShotDecline >= 5 {
		reasons = append(reasons, "Shooting from worse positions — shot quality declining")
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

	// 6. Position-specific speciality reliability (0-10)
	//
	//  GK:  Goals conceded per game close to 0.9/game is the reliable keeper zone —
	//       not so low that they only face routine saves, not so high that defence is
	//       chaotic. Scores 10 at 0.9, drops off steeply in either direction.
	//
	//  DEF: Duel and tackle win rates near 52-58% signal a dependably combative
	//       defender — aggressive enough to win challenges, controlled enough not to
	//       lunge. Both rates contribute (60% duels / 40% tackles).
	//
	//  MID: Key passes per game near 1.5 = reliable creative presence without being
	//       the team's star. Too few = limited; too many = not benchwarmer territory.
	//
	//  FWD: Shots per game near 2.5 = consistent threat without being prolific.
	//       Tests the keeper every game but isn't hogging the ball or over-shooting.
	var specialityScore float64
	switch p.Position {
	case "GK":
		gcPerGame := float64(p.GoalsConceded) / games
		specialityScore = math.Max(0, 10-math.Abs(gcPerGame-0.9)*7)
	case "DEF":
		duelRate := 0.5
		if p.DuelsTotal > 0 {
			duelRate = float64(p.DuelsWon) / float64(p.DuelsTotal)
		}
		tackleRate := 0.5
		if p.TacklesTotal > 0 {
			tackleRate = float64(p.TacklesWon) / float64(p.TacklesTotal)
		}
		specialityScore = math.Max(0, 10-math.Abs(duelRate-0.55)*20)*0.60 +
			math.Max(0, 10-math.Abs(tackleRate-0.55)*20)*0.40
	case "MID":
		kpPerGame := float64(p.KeyPasses) / games
		specialityScore = math.Max(0, 10-math.Abs(kpPerGame-1.5)*4)
	default: // FWD
		shotsPerGame := float64(p.TotalShots) / games
		specialityScore = math.Max(0, 10-math.Abs(shotsPerGame-2.5)*2.5)
	}

	// Weighted composite by position
	switch p.Position {
	case "GK":
		// specialityScore (gc/game) replaces some outputReliability weight — goals
		// conceded per game is a more granular reliability signal than raw save rate.
		score = availScore*0.25 + formConsistency*0.22 + outputReliability*0.25 +
			specialityScore*0.15 + discipline*0.13
	case "DEF":
		// specialityScore (duel/tackle rates) captures physical defensive consistency
		// that G+A proximity can't measure.
		score = availScore*0.22 + formConsistency*0.22 + outputReliability*0.15 +
			passReliability*0.15 + specialityScore*0.13 + discipline*0.13
	case "MID":
		// specialityScore (key passes/game) captures steady creative contribution.
		score = availScore*0.18 + formConsistency*0.22 + outputReliability*0.20 +
			passReliability*0.18 + specialityScore*0.12 + discipline*0.10
	default: // FWD
		// specialityScore (shots/game) captures reliable goal threat beyond just G+A.
		score = availScore*0.18 + formConsistency*0.22 + outputReliability*0.25 +
			passReliability*0.13 + specialityScore*0.12 + discipline*0.10
	}
	// Keep more precision to reduce artificial ties after normalization.
	score = math.Round(score*1000) / 1000

	// Position-specific labels make the benchwarmer list feel more meaningful to
	// users — a "Wall Between the Posts" reads very differently from a "Rock Solid"
	// label applied generically across all positions.
	switch p.Position {
	case "GK":
		switch {
		case score >= 7.5:
			label = "Wall Between the Posts"
		case score >= 5.5:
			label = "Reliable Stopper"
		case score >= 4.0:
			label = "Rotation Keeper"
		default:
			label = ""
		}
	case "DEF":
		switch {
		case score >= 7.5:
			label = "Defensive Pillar"
		case score >= 5.5:
			label = "Solid Defender"
		case score >= 4.0:
			label = "Rotation Defender"
		default:
			label = ""
		}
	case "MID":
		switch {
		case score >= 7.5:
			label = "Engine Room"
		case score >= 5.5:
			label = "Steady Midfielder"
		case score >= 4.0:
			label = "Rotation Mid"
		default:
			label = ""
		}
	default: // FWD
		switch {
		case score >= 7.5:
			label = "Reliable Striker"
		case score >= 5.5:
			label = "Impact Substitute"
		case score >= 4.0:
			label = "Rotation Forward"
		default:
			label = ""
		}
	}
	return
}
