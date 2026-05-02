package services

import "prediplay/backend/models"

// GetPlayer returns the player record with the given ID.
func (s *PredictionService) GetPlayer(playerID uint) (models.Player, error) {
	var p models.Player
	return p, s.db.First(&p, playerID).Error
}

// GetAllPlayers returns players filtered by league, position, and team name substring.
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
