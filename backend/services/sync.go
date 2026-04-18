package services

import (
	"fmt"
	"prediplay/backend/models"
	"sort"
	"sync"
	"time"

	"gorm.io/gorm/clause"
)

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
