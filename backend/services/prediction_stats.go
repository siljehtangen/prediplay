package services

import (
	"math"
	"prediplay/backend/models"
	"sort"
	"strings"
)

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

// statTotals holds accumulated raw stat values from a slice of PlayerStat.
type statTotals struct {
	mins, goals, assists, shots, shotsOT, keyPasses uint
	totalPasses, accPasses, duelsWon, duelsTotal     uint
	tacklesWon, tacklesTotal                         uint
	yellowCards, redCards, saves, gconceded          uint
	xg, xa, rating                                   float64
	ratedGames, gamesPlayed                          int
}

func accumulateStats(stats []models.PlayerStat) statTotals {
	var t statTotals
	for _, st := range stats {
		if st.MinutesPlayed > 0 {
			t.gamesPlayed++
		}
		t.mins += st.MinutesPlayed
		t.goals += st.Goals
		t.assists += st.GoalAssist
		t.xg += st.ExpectedGoals
		t.xa += st.ExpectedAssists
		t.shots += st.TotalShots
		t.shotsOT += st.ShotsOnTarget
		t.keyPasses += st.KeyPass
		t.totalPasses += st.TotalPass
		t.accPasses += st.AccuratePass
		t.duelsWon += st.DuelWon
		t.duelsTotal += st.DuelWon + st.DuelLost
		t.tacklesWon += st.WonTackle
		t.tacklesTotal += st.TotalTackle
		t.yellowCards += st.YellowCard
		t.redCards += st.RedCard
		t.saves += st.Saves
		t.gconceded += st.GoalsConceded
		if st.Rating > 0 {
			t.rating += st.Rating
			t.ratedGames++
		}
	}
	return t
}

// aggregateOverall computes full-season totals into the Player's main stat fields.
func aggregateOverall(p *models.Player, stats []models.PlayerStat) {
	t := accumulateStats(stats)
	p.GamesPlayed = t.gamesPlayed
	p.MinutesPlayed = int(t.mins)
	p.Goals = int(t.goals)
	p.Assists = int(t.assists)
	p.XG = t.xg
	p.XA = t.xa
	p.TotalShots = int(t.shots)
	p.ShotsOnTarget = int(t.shotsOT)
	p.KeyPasses = int(t.keyPasses)
	p.TotalPasses = int(t.totalPasses)
	p.AccuratePasses = int(t.accPasses)
	p.DuelsWon = int(t.duelsWon)
	p.DuelsTotal = int(t.duelsTotal)
	p.TacklesWon = int(t.tacklesWon)
	p.TacklesTotal = int(t.tacklesTotal)
	p.YellowCards = int(t.yellowCards)
	p.RedCards = int(t.redCards)
	p.Saves = int(t.saves)
	p.GoalsConceded = int(t.gconceded)
	if t.ratedGames > 0 {
		p.FormScore = t.rating / float64(t.ratedGames)
	} else if p.FormScore == 0 {
		p.FormScore = 6.0
	}
}

// aggregateRecent computes stats from the last 3 played games into the Player's Recent* fields.
func aggregateRecent(p *models.Player, stats []models.PlayerStat) {
	t := accumulateStats(stats)
	p.RecentMinutes = int(t.mins)
	p.RecentGoals = int(t.goals)
	p.RecentAssists = int(t.assists)
	p.RecentXG = t.xg
	p.RecentXA = t.xa
	p.RecentTotalShots = int(t.shots)
	p.RecentShotsOnTarget = int(t.shotsOT)
	p.RecentKeyPasses = int(t.keyPasses)
	p.RecentTotalPasses = int(t.totalPasses)
	p.RecentAccuratePasses = int(t.accPasses)
	p.RecentDuelsWon = int(t.duelsWon)
	p.RecentDuelsTotal = int(t.duelsTotal)
	p.RecentTacklesWon = int(t.tacklesWon)
	p.RecentTacklesTotal = int(t.tacklesTotal)
	p.RecentYellowCards = int(t.yellowCards)
	p.RecentRedCards = int(t.redCards)
	p.RecentSaves = int(t.saves)
	p.RecentGoalsConceded = int(t.gconceded)
	if t.ratedGames > 0 {
		p.RecentFormScore = t.rating / float64(t.ratedGames)
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
