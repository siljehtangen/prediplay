package services

import (
	"math"
	"prediplay/backend/models"
	"sort"
)

// minMinutesOverall is the minimum season minutes required for overall-mode analysis
// (≈9 games × 30 min/game).
const minMinutesOverall = 270

// minMinutesRecent is the minimum season minutes required for recent-mode analysis
// (≈1 full game).
const minMinutesRecent = 45

// GetRedFlags passes the original (full-stat) player to calcRedFlag so it can
// compare recent vs overall as a true decline signal. The scoringView eligibility
// filter (3 games played) is applied manually.
// GetRedFlags returns players with declining performance signals, filtered by league,
// position, and time filter.
func (s *PredictionService) GetRedFlags(league, position, timeFilter string) ([]models.RedFlagPlayer, error) {
	minMinutes := minMinutesOverall
	if timeFilter != "overall" {
		minMinutes = minMinutesRecent
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

			// Exclude players already showing meaningful red-flag decline.
			rfScore, _, _, _ := calcRedFlag(p)
			if rfScore >= 4.0 {
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
