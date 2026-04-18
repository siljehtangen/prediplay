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

	type nextFixture struct {
		opponent string
		isHome   bool
	}
	nextFixtureByTeam := map[uint]nextFixture{}
	today := time.Now().Format("2006-01-02")
	nextWeek := time.Now().AddDate(0, 0, 14).Format("2006-01-02")
	if events, err := s.client.GetEvents(today, nextWeek, "", ""); err == nil {
		sort.Slice(events, func(i, j int) bool {
			return events[i].Date.Before(events[j].Date)
		})
		for _, ev := range events {
			if _, done := nextFixtureByTeam[ev.HomeTeamID]; !done {
				nextFixtureByTeam[ev.HomeTeamID] = nextFixture{opponent: ev.AwayTeam.Name, isHome: true}
			}
			if _, done := nextFixtureByTeam[ev.AwayTeamID]; !done {
				nextFixtureByTeam[ev.AwayTeamID] = nextFixture{opponent: ev.HomeTeam.Name, isHome: false}
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
			fix := nextFixtureByTeam[p.TeamID]
			updates[i] = s.enrichAndCompute(p, fix.opponent, fix.isHome)
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

// enrichAndCompute fetches all stats for a player and computes the aggregate fields
// in-memory. It does not write to the DB; the caller can batch persist the result.
func (s *PredictionService) enrichAndCompute(p models.Player, nextOpponent string, isHome bool) models.Player {
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

	if len(played) > 0 {
		d := played[0].Event.EventDate
		if len(d) > 10 {
			d = d[:10]
		}
		p.LastMatchDate = d
	}

	p.NextOpponent = nextOpponent
	p.IsHome = isHome
	p.OpponentScore = playerVsOpponentScore(stats, nextOpponent)

	return p
}

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

		ordering := make([]models.PlayerPrediction, len(preds))
		copy(ordering, preds)
		normalizePlayerPredictedScoresByPosition(ordering)

		// Apply position quota (GK=1 DEF=2 MID=3 FWD=3) using normalized scores so
		// each position competes fairly within itself before the cross-position cut.
		// This is what prevents one position from flooding the top-9 list.
		ordering = pickByPositionQuota(ordering,
			func(p models.PlayerPrediction) string { return canonicalPosition(p.Player.Position) },
			func(a, b models.PlayerPrediction) bool { return a.PredictedScore > b.PredictedScore },
		)

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

// GetRedFlags passes the original (full-stat) player to calcRedFlag so it can
// compare recent vs overall as a true decline signal. The scoringView eligibility
// filter (3 games played) is applied manually.
func (s *PredictionService) GetRedFlags(league, position, timeFilter string) ([]models.RedFlagPlayer, error) {
	minMinutes := 270
	if timeFilter != "overall" {
		minMinutes = 45
	}
	players, err := s.loadPlayers(league, position, minMinutes)
	if err != nil {
		return nil, err
	}

	var result []models.RedFlagPlayer
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
		result = pickByPositionQuota(result,
			func(f models.RedFlagPlayer) string { return canonicalPosition(f.Player.Position) },
			func(a, b models.RedFlagPlayer) bool { return a.RedFlagScore > b.RedFlagScore },
		)
	} else if len(result) > 9 {
		result = result[:9]
	}
	return result, nil
}

func (s *PredictionService) GetBenchwarmers(league, position, timeFilter string) ([]models.BenchwarmerPlayer, error) {
	minMinutes := 270
	if timeFilter != "overall" {
		minMinutes = 45
	}
	players, err := s.loadPlayers(league, position, minMinutes)
	if err != nil {
		return nil, err
	}

	var result []models.BenchwarmerPlayer

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
		result = pickByPositionQuota(result,
			func(bw models.BenchwarmerPlayer) string { return canonicalPosition(bw.Player.Position) },
			func(a, b models.BenchwarmerPlayer) bool { return a.ConsistencyScore > b.ConsistencyScore },
		)
	} else if len(result) > 9 {
		result = result[:9]
	}
	return result, nil
}

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

func (s *PredictionService) loadPlayers(league, position string, minMinutes ...int) ([]models.Player, error) {
	query := s.db.Model(&models.Player{})
	if len(minMinutes) > 0 && minMinutes[0] > 0 {
		query = query.Where("minutes_played >= ?", minMinutes[0])
	}
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
