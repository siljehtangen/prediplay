package bzzoiro

import "prediplay/backend/models"

// GetPredictions returns match predictions, optionally restricted to upcoming fixtures.
func (c *Client) GetPredictions(upcoming bool) ([]models.Prediction, error) {
	params := map[string]string{}
	if upcoming {
		params["upcoming"] = "true"
	}
	raw, err := fetchAll[rawPrediction](c, "/api/predictions/", params)
	if err != nil {
		return nil, err
	}
	out := make([]models.Prediction, len(raw))
	for i, r := range raw {
		out[i] = models.Prediction{
			ID:              r.ID,
			HomeTeam:        r.Event.HomeTeam,
			AwayTeam:        r.Event.AwayTeam,
			ProbHomeWin:     r.ProbHomeWin,
			ProbDraw:        r.ProbDraw,
			ProbAwayWin:     r.ProbAwayWin,
			PredictedResult: r.PredictedResult,
			ProbOver25:      r.ProbOver25,
			ProbBttsYes:     r.ProbBttsYes,
			Confidence:      r.Confidence,
			ModelVersion:    r.ModelVersion,
		}
	}
	return out, nil
}
