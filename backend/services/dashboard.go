package services

import (
	"fmt"
	"math"
	"prediplay/backend/models"
	"strings"
	"time"
)

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
