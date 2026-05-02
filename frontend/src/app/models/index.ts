export interface League {
  id: number;
  name: string;
  country: string;
  active: boolean;
}

export interface Team {
  id: number;
  name: string;
  country: string;
  league_id: number;
}

export interface Player {
  id: number;
  api_id: number;
  name: string;
  short_name: string;
  team_id: number;
  team_name: string;
  league: string;
  position: string;
  jersey_number: number;
  height: number;
  date_of_birth: string;
  nationality: string;
  market_value: number;
  minutes_played: number;
  games_played: number;
  goals: number;
  assists: number;
  xG: number;
  xA: number;
  form_score: number;
  total_shots: number;
  shots_on_target: number;
  key_passes: number;
  total_passes: number;
  accurate_passes: number;
  duels_won: number;
  duels_total: number;
  tackles_won: number;
  tackles_total: number;
  yellow_cards: number;
  red_cards: number;
  saves: number;
  goals_conceded: number;

  recent_games_played: number;
  recent_minutes: number;
  recent_goals: number;
  recent_assists: number;
  recent_xg: number;
  recent_xa: number;
  recent_total_shots: number;
  recent_shots_on_target: number;
  recent_key_passes: number;
  recent_total_passes: number;
  recent_accurate_passes: number;
  recent_duels_won: number;
  recent_duels_total: number;
  recent_tackles_won: number;
  recent_tackles_total: number;
  recent_yellow_cards: number;
  recent_red_cards: number;
  recent_saves: number;
  recent_goals_conceded: number;
  recent_form_score: number;
}

export interface Event {
  id: number;
  league_id: number;
  home_team: Team;
  away_team: Team;
  date: string;
  status: string;
}

export interface StatEvent {
  id: number;
  home_team: string;
  away_team: string;
  event_date: string;
  home_score: number;
  away_score: number;
}

export interface PlayerStat {
  event: StatEvent;
  minutes_played: number;
  rating: number;
  goals: number;
  goal_assist: number;
  expected_goals: number;
  expected_assists: number;
  total_shots: number;
  shots_on_target: number;
  total_pass: number;
  accurate_pass: number;
  key_pass: number;
  touches: number;
  duel_won: number;
  duel_lost: number;
  total_tackle: number;
  won_tackle: number;
  yellow_card: number;
  red_card: number;
  saves: number;
  goals_conceded: number;
}

/** Match prediction from /api/predictions/ */
export interface Prediction {
  id: number;
  home_team: string;
  away_team: string;
  prob_home_win: number;
  prob_draw: number;
  prob_away_win: number;
  predicted_result: string;
  prob_over_25: number;
  prob_btts_yes: number;
  confidence: number;
  model_version: string;
}

export interface PlayerPrediction {
  player: Player;
  predicted_score: number;
  risk_level: 'low' | 'medium' | 'high';
  hidden_gem: boolean;
  hidden_gem_reasons?: string[];
  form_contribution: number;
  threat_contribution: number;
  opponent_difficulty: number;
  minutes_likelihood: number;
  defensive_contribution: number;
  next_event?: Event;
}

export interface MomentumGame {
  date: string;
  opponent: string;
  score: number;
  goals: number;
  assists: number;
  minutes: number;
}

export interface MomentumData {
  player: Player;
  games: MomentumGame[];
  trend: 'rising' | 'falling' | 'stable';
}

export interface SynergyResult {
  players: Player[];
  total_predicted: number;
  synergy_bonus: number;
  synergy_score: number;
}

export interface RedFlagPlayer {
  player: Player;
  red_flag_score: number;
  form_decline: number;
  output_drop: number;
  reasons: string[];
}

export interface BenchwarmerPlayer {
  player: Player;
  consistency_score: number;
  label: 'Rock Solid' | 'Steady Option' | 'Rotation Pick';
}

export interface DashboardLeague {
  name: string;
  top_players: PlayerPrediction[];
  red_flags: RedFlagPlayer[];
}

export const ALL_LEAGUES = ['Premier League', 'La Liga', 'Bundesliga', 'Serie A', 'Ligue 1'] as const;

export function scoreClass(risk: string): string {
  return risk === 'low' ? 'list-score--high' : risk === 'medium' ? 'list-score--medium' : 'list-score--low';
}
