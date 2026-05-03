package services

import (
	"fmt"
	"log"
	"math"
	"prediplay/backend/models"
	"strings"
	"time"
)

// dashboardTopN is the number of top players and red-flag players shown per league.
const dashboardTopN = 3

// momentumGameLimit is the maximum number of recent games included in a momentum series.
const momentumGameLimit = 10

var dashboardLeagueList = []string{"Premier League", "La Liga", "Bundesliga", "Serie A", "Ligue 1"}

// GetDashboard returns top predictions and red flags for each supported league,
// fetched in parallel. Individual league errors are logged and produce empty results.
func (s *PredictionService) GetDashboard(timeFilter string) ([]models.DashboardLeague, error) {
	type leagueResult struct {
		idx      int
		top      []models.PlayerPrediction
		redFlags []models.RedFlagPlayer
	}
	ch := make(chan leagueResult, len(dashboardLeagueList))
	for i, league := range dashboardLeagueList {
		go func(idx int, l string) {
			top, err := s.GetTopPredictions(l, "", "", timeFilter)
			if err != nil {
				log.Printf("GetDashboard: GetTopPredictions for %s: %v", l, err)
			}
			flags, err := s.GetRedFlags(l, "", timeFilter)
			if err != nil {
				log.Printf("GetDashboard: GetRedFlags for %s: %v", l, err)
			}
			if len(top) > dashboardTopN {
				top = top[:dashboardTopN]
			}
			if len(flags) > dashboardTopN {
				flags = flags[:dashboardTopN]
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

// GetMomentum returns a player's performance trend over their most recent games.
func (s *PredictionService) GetMomentum(playerID uint) (*models.MomentumData, error) {
	var player models.Player
	if err := s.db.First(&player, playerID).Error; err != nil {
		return nil, fmt.Errorf("player not found: %w", err)
	}

	stats, err := s.client.GetPlayerStats(playerID)
	if err != nil {
		log.Printf("GetMomentum: failed to fetch stats for player %d: %v", playerID, err)
	}

	played := playedGames(stats)
	sortByDateDesc(played)
	if len(played) > momentumGameLimit {
		played = played[:momentumGameLimit]
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
		// Split into two equal halves, skipping the middle game when n is odd so both
		// halves have the same size and the comparison is symmetric.
		half := n / 2
		recent, older := 0.0, 0.0
		for i := 0; i < half; i++ {
			recent += games[i].Score           // most recent half (games[0] is newest)
			older += games[n-half+i].Score     // oldest half
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

// GetSynergy returns the combined prediction score and a position-diversity bonus
// for the given set of player IDs.
func (s *PredictionService) GetSynergy(playerIDs []uint) (*models.SynergyResult, error) {
	players := make([]models.Player, 0, len(playerIDs))
	for _, id := range playerIDs {
		var p models.Player
		if err := s.db.First(&p, id).Error; err == nil {
			players = append(players, p)
		}
	}
	if len(players) == 0 {
		return nil, fmt.Errorf("no players found for the provided IDs")
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
