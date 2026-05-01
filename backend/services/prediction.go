package services

import (
	"fmt"
	"prediplay/backend/models"
	"sort"

	"prediplay/backend/bzzoiro"

	"gorm.io/gorm"
)

type PredictionService struct {
	db     *gorm.DB
	client *bzzoiro.Client
}

func NewPredictionService(db *gorm.DB, client *bzzoiro.Client) *PredictionService {
	return &PredictionService{db: db, client: client}
}

func (s *PredictionService) GetPlayerPrediction(playerID uint) (*models.PlayerPrediction, error) {
	var player models.Player
	if err := s.db.First(&player, playerID).Error; err != nil {
		return nil, fmt.Errorf("player not found: %w", err)
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
