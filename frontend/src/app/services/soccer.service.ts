import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import {
  League, Team, Event, Prediction, Player,
  PlayerPrediction, PlayerStat, MomentumData, SynergyResult,
  PredictionWeights, DEFAULT_WEIGHTS, RedFlagPlayer, BenchwarmerPlayer, DashboardLeague
} from '../models';

@Injectable({ providedIn: 'root' })
export class SoccerService {
  constructor(private http: HttpClient) {}

  getLeagues(): Observable<League[]> {
    return this.http.get<League[]>('/api/leagues');
  }

  getTeams(country = ''): Observable<Team[]> {
    const params = country ? `?country=${encodeURIComponent(country)}` : '';
    return this.http.get<Team[]>(`/api/teams${params}`);
  }

  getEvents(dateFrom = '', dateTo = '', league = '', status = ''): Observable<Event[]> {
    const p = new URLSearchParams();
    if (dateFrom) p.set('date_from', dateFrom);
    if (dateTo)   p.set('date_to',   dateTo);
    if (league)   p.set('league',    league);
    if (status)   p.set('status',    status);
    const qs = p.toString() ? `?${p}` : '';
    return this.http.get<Event[]>(`/api/events${qs}`);
  }

  getLive(): Observable<Event[]> {
    return this.http.get<Event[]>('/api/live');
  }

  getPredictions(upcoming = true): Observable<Prediction[]> {
    return this.http.get<Prediction[]>(`/api/predictions?upcoming=${upcoming}`);
  }

  getPlayers(league = '', position = '', team = ''): Observable<Player[]> {
    const p = new URLSearchParams();
    if (league)   p.set('league',   league);
    if (position) p.set('position', position);
    if (team)     p.set('team',     team);
    const qs = p.toString() ? `?${p}` : '';
    return this.http.get<Player[]>(`/api/players${qs}`);
  }

  getPlayerPrediction(playerId: number, weights = DEFAULT_WEIGHTS): Observable<PlayerPrediction> {
    return this.http.get<PlayerPrediction>(
      `/api/predict/player/${playerId}?weights=${this.weightsParam(weights)}`
    );
  }

  getTopPredictions(league = '', position = '', hiddenGem = false, timeFilter = 'recent'): Observable<PlayerPrediction[]> {
    const p = new URLSearchParams({ time_filter: timeFilter });
    if (league)    p.set('league',     league);
    if (position)  p.set('position',   position);
    if (hiddenGem) p.set('hidden_gem', 'true');
    return this.http.get<PlayerPrediction[]>(`/api/predict/top?${p}`);
  }

  getPlayerStats(playerId: number): Observable<PlayerStat[]> {
    return this.http.get<PlayerStat[]>(`/api/players/${playerId}/stats`);
  }

  getPlayerPhotoUrl(playerId: number): string {
    return `/api/players/${playerId}/photo`;
  }

  getDashboard(timeFilter = 'recent'): Observable<DashboardLeague[]> {
    return this.http.get<DashboardLeague[]>(`/api/dashboard?time_filter=${timeFilter}`);
  }

  getRedFlags(league = '', position = '', timeFilter = 'recent'): Observable<RedFlagPlayer[]> {
    const p = new URLSearchParams({ time_filter: timeFilter });
    if (league)   p.set('league',   league);
    if (position) p.set('position', position);
    return this.http.get<RedFlagPlayer[]>(`/api/predict/redflags?${p}`);
  }

  getBenchwarmers(league = '', position = '', timeFilter = 'recent'): Observable<BenchwarmerPlayer[]> {
    const p = new URLSearchParams({ time_filter: timeFilter });
    if (league)   p.set('league',   league);
    if (position) p.set('position', position);
    return this.http.get<BenchwarmerPlayer[]>(`/api/predict/benchwarmers?${p}`);
  }

  getMomentum(playerId: number): Observable<MomentumData> {
    return this.http.get<MomentumData>(`/api/predict/momentum?player=${playerId}`);
  }

  getSynergy(playerIds: number[], weights = DEFAULT_WEIGHTS): Observable<SynergyResult> {
    return this.http.get<SynergyResult>(
      `/api/predict/synergy?players=${playerIds.join(',')}&weights=${this.weightsParam(weights)}`
    );
  }

  private weightsParam(w: PredictionWeights): string {
    return `form:${w.form},threat:${w.threat},opponent:${w.opponent},minutes:${w.minutes},home_away:${w.home_away}`;
  }
}
