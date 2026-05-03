package services

import (
	"math"
	"prediplay/backend/models"
	"sort"
)

// per90 converts a raw minutes total to a per-90-minutes denominator, floored at 1
// to avoid division by zero for players with minimal recorded time.
func per90(minutes int) float64 {
	return math.Max(1, float64(minutes)/90.0)
}

// safeRate computes won/total as a fraction, returning 0.50 (neutral) when total is 0.
// Used for duel win rate, tackle win rate, and similar "attempts → success" ratios.
func safeRate(won, total int) float64 {
	if total == 0 {
		return 0.50
	}
	return float64(won) / float64(total)
}

// positionQuota defines how many top players to pick per position when no
// position filter is active. Matches a typical attacking lineup shape (4-3-3 / 4-2-3-1).
// topPredictionsLimit is the maximum number of predictions returned when no position
// filter is active. It equals the sum of positionQuota values (1+2+3+3).
const topPredictionsLimit = 9

var positionQuota = map[string]int{
	"GK":  1,
	"DEF": 2,
	"MID": 3,
	"FWD": 3,
}

func pickByPositionQuota[T any](items []T, getPos func(T) string, less func(a, b T) bool) []T {
	byPos := map[string][]T{"GK": {}, "DEF": {}, "MID": {}, "FWD": {}}
	for _, item := range items {
		pos := getPos(item)
		if _, ok := byPos[pos]; !ok {
			pos = "FWD"
		}
		byPos[pos] = append(byPos[pos], item)
	}
	result := make([]T, 0, topPredictionsLimit)
	for pos, quota := range positionQuota {
		group := byPos[pos]
		sort.Slice(group, func(i, j int) bool { return less(group[i], group[j]) })
		if len(group) > quota {
			group = group[:quota]
		}
		result = append(result, group...)
	}
	sort.Slice(result, func(i, j int) bool { return less(result[i], result[j]) })
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

func canonicalPosition(pos string) string {
	switch pos {
	case "GK", "DEF", "MID", "FWD":
		return pos
	default:
		return "FWD"
	}
}

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
	idx := max(0, min(len(sortedAsc)-1, int(math.Floor(frac*float64(len(sortedAsc)-1)))))
	return sortedAsc[idx]
}

func normalizeScore(score, low, high float64) float64 {
	if high <= low {
		return 5.0
	}
	n := (score - low) / (high - low) * 10.0
	if n < 0 {
		n = 0
	}
	if n > 10 {
		n = 10
	}
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

		// Use robust tail bounds so the top doesn't get flattened into lots of 10s.
		low := percentile(scoresAsc, 0.005)
		high := percentile(scoresAsc, 0.995)

		for _, idx := range indices {
			norm := normalizeScore(preds[idx].PredictedScore, low, high)
			preds[idx].PredictedScore = norm
			preds[idx].RiskLevel = riskLevelFromPredictedScore(norm)
		}
	}
}

