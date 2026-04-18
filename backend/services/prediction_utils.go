package services

import (
	"math"
	"prediplay/backend/models"
	"sort"
)

// positionQuota defines how many top players to pick per position when no
// position filter is active. Matches a typical attacking lineup shape (4-3-3 / 4-2-3-1).
var positionQuota = map[string]int{
	"GK":  1,
	"DEF": 2,
	"MID": 3,
	"FWD": 3,
}

// pickByPositionQuota selects items using per-position quotas (GK=1 DEF=2 MID=3 FWD=3)
// so no single position can flood the top-N list.
// getPos extracts the canonical position; less returns true when a should rank before b.
func pickByPositionQuota[T any](items []T, getPos func(T) string, less func(a, b T) bool) []T {
	byPos := map[string][]T{"GK": {}, "DEF": {}, "MID": {}, "FWD": {}}
	for _, item := range items {
		pos := getPos(item)
		if _, ok := byPos[pos]; !ok {
			pos = "FWD"
		}
		byPos[pos] = append(byPos[pos], item)
	}
	result := make([]T, 0, 9)
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

		for _, idx := range indices {
			flags[idx].RedFlagScore = normalizeScore(flags[idx].RedFlagScore, low, high)
		}
	}
}
