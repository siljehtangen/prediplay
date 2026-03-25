import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { MatCardModule } from '@angular/material/card';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { forkJoin, of } from 'rxjs';
import { catchError } from 'rxjs/operators';
import { SoccerService } from '../../services/soccer.service';
import { PlayerPrediction, RedFlagPlayer, MomentumData, MomentumGame } from '../../models';

const LEAGUES = ['Premier League', 'La Liga', 'Bundesliga', 'Serie A', 'Ligue 1'];

interface LeagueData {
  name: string;
  topPlayers: PlayerPrediction[];
  redFlags: RedFlagPlayer[];
  topMomentum?: MomentumData;
  flagMomentum?: MomentumData;
}

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [CommonModule, RouterLink, MatCardModule, MatButtonModule, MatIconModule],
  templateUrl: './dashboard.component.html',
  styleUrl: './dashboard.component.scss'
})
export class DashboardComponent implements OnInit {
  leagues: LeagueData[] = [];
  loading = true;
  timeFilter: 'recent' | 'overall' = 'recent';

  constructor(private soccer: SoccerService) {}

  ngOnInit() { this.loadData(); }

  loadData() {
    this.loading = true;
    const top$ = LEAGUES.map(l =>
      this.soccer.getTopPredictions(l, '', false, this.timeFilter).pipe(catchError(() => of([])))
    );
    const flags$ = LEAGUES.map(l =>
      this.soccer.getRedFlags(l, '', this.timeFilter).pipe(catchError(() => of([])))
    );
    forkJoin([...top$, ...flags$]).subscribe(results => {
      const topPlayers = LEAGUES.map((_, i) => (results[i] as PlayerPrediction[]).slice(0, 3));
      const redFlags = LEAGUES.map((_, i) => (results[i + LEAGUES.length] as RedFlagPlayer[]).slice(0, 3));

      const momentum$ = LEAGUES.map((_, i) => forkJoin({
        top: topPlayers[i].length > 0
          ? this.soccer.getMomentum(topPlayers[i][0].player.id).pipe(catchError(() => of(null)))
          : of(null),
        flag: redFlags[i].length > 0
          ? this.soccer.getMomentum(redFlags[i][0].player.id).pipe(catchError(() => of(null)))
          : of(null),
      }));

      forkJoin(momentum$).subscribe(momentumResults => {
        this.leagues = LEAGUES.map((name, i) => ({
          name,
          topPlayers: topPlayers[i],
          redFlags: redFlags[i],
          topMomentum: momentumResults[i].top ?? undefined,
          flagMomentum: momentumResults[i].flag ?? undefined,
        })).filter(l => l.topPlayers.length > 0 || l.redFlags.length > 0);
        this.loading = false;
      });
    });
  }

  setTimeFilter(f: 'recent' | 'overall') {
    this.timeFilter = f;
    this.loadData();
  }

  scoreClass(risk: string) {
    return risk === 'low' ? 'score-circle--high' : risk === 'medium' ? 'score-circle--medium' : 'score-circle--low';
  }

  donutStyle(pred: PlayerPrediction): string {
    const pct = (pred.predicted_score * 10).toFixed(1);
    return `conic-gradient(#6c63ff 0% ${pct}%, rgba(108,99,255,0.12) ${pct}% 100%)`;
  }

  dangerStyle(score: number): string {
    const pct = score * 10;
    return `conic-gradient(#f44336 0% ${pct.toFixed(1)}%, rgba(244,67,54,0.1) ${pct.toFixed(1)}% 100%)`;
  }

  barWidth(score: number): number {
    return Math.min(100, score * 10);
  }

  sparklinePath(games: MomentumGame[], w = 200, h = 36): string {
    if (!games || games.length < 2) return '';
    // Games are sorted most-recent-first; take newest 3, then reverse for left→right chronological
    const pts = games.slice(0, 3).reverse().map(g => g.score);
    const min = Math.min(...pts);
    const max = Math.max(...pts);
    const range = max - min || 0.1;
    return pts.map((s, i) => {
      const x = (i / (pts.length - 1)) * w;
      const y = h - ((s - min) / range) * (h - 6) - 3;
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    }).join(' ');
  }

  trendColor(trend: string): string {
    return trend === 'rising' ? '#81c784' : trend === 'falling' ? '#f44336' : '#ffd54f';
  }

  trendIcon(trend: string): string {
    return trend === 'rising' ? 'trending_up' : trend === 'falling' ? 'trending_down' : 'trending_flat';
  }
}
