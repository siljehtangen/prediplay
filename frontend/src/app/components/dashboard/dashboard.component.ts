import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { MatCardModule } from '@angular/material/card';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { forkJoin } from 'rxjs';
import { SoccerService } from '../../services/soccer.service';
import { PlayerPrediction, RedFlagPlayer, Event, Prediction } from '../../models';

interface LeagueData {
  name: string;
  topPlayers: PlayerPrediction[];
  redFlags: RedFlagPlayer[];
}

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [CommonModule, RouterLink, MatCardModule, MatButtonModule, MatIconModule, MatProgressBarModule],
  templateUrl: './dashboard.component.html',
  styleUrl: './dashboard.component.scss'
})
export class DashboardComponent implements OnInit {
  leagues: LeagueData[] = [];
  events: Event[] = [];
  predictions: Prediction[] = [];
  loading = true;

  constructor(private soccer: SoccerService) {}

  ngOnInit() {
    forkJoin({
      pl_top:   this.soccer.getTopPredictions('Premier League'),
      pl_flags: this.soccer.getRedFlags('Premier League'),
      ll_top:   this.soccer.getTopPredictions('La Liga'),
      ll_flags: this.soccer.getRedFlags('La Liga'),
      bl_top:   this.soccer.getTopPredictions('Bundesliga'),
      bl_flags: this.soccer.getRedFlags('Bundesliga'),
      sa_top:   this.soccer.getTopPredictions('Serie A'),
      sa_flags: this.soccer.getRedFlags('Serie A'),
      l1_top:   this.soccer.getTopPredictions('Ligue 1'),
      l1_flags: this.soccer.getRedFlags('Ligue 1'),
      events:   this.soccer.getEvents(),
      preds:    this.soccer.getPredictions(true),
    }).subscribe({
      next: (r) => {
        this.leagues = [
          { name: 'Premier League', topPlayers: r.pl_top.slice(0, 3), redFlags: r.pl_flags.slice(0, 3) },
          { name: 'La Liga',        topPlayers: r.ll_top.slice(0, 3), redFlags: r.ll_flags.slice(0, 3) },
          { name: 'Bundesliga',     topPlayers: r.bl_top.slice(0, 3), redFlags: r.bl_flags.slice(0, 3) },
          { name: 'Serie A',        topPlayers: r.sa_top.slice(0, 3), redFlags: r.sa_flags.slice(0, 3) },
          { name: 'Ligue 1',        topPlayers: r.l1_top.slice(0, 3), redFlags: r.l1_flags.slice(0, 3) },
        ];
        this.events     = r.events.slice(0, 6);
        this.predictions = r.preds;
        this.loading = false;
      },
      error: () => { this.loading = false; }
    });
  }

  getPrediction(homeTeam: string | undefined, awayTeam: string | undefined): Prediction | undefined {
    if (!homeTeam || !awayTeam) return undefined;
    return this.predictions.find(p =>
      p.home_team?.toLowerCase() === homeTeam.toLowerCase() &&
      p.away_team?.toLowerCase() === awayTeam.toLowerCase()
    );
  }

  scoreClass(risk: string) {
    return risk === 'low' ? 'score-circle--high' : risk === 'medium' ? 'score-circle--medium' : 'score-circle--low';
  }

  formatDate(d: string): string {
    return new Date(d).toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric', month: 'short' });
  }
}
