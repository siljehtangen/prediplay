import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink, ActivatedRoute } from '@angular/router';
import { FormsModule } from '@angular/forms';
import { MatCardModule } from '@angular/material/card';
import { MatSelectModule } from '@angular/material/select';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatFormFieldModule } from '@angular/material/form-field';
import { forkJoin } from 'rxjs';
import { SoccerService } from '../../services/soccer.service';
import { League, PlayerPrediction } from '../../models';

const ALL_LEAGUES = ['Premier League', 'La Liga', 'Bundesliga', 'Serie A', 'Ligue 1'];

interface LeagueGroup {
  name: string;
  players: PlayerPrediction[];
}

@Component({
  selector: 'app-top-players',
  standalone: true,
  imports: [CommonModule, RouterLink, FormsModule, MatCardModule, MatSelectModule, MatButtonModule,
    MatIconModule, MatProgressBarModule, MatFormFieldModule],
  templateUrl: './top-players.component.html',
  styleUrl: './top-players.component.scss'
})
export class TopPlayersComponent implements OnInit {
  leagues: League[] = [];
  predictions: PlayerPrediction[] = [];
  leagueGroups: LeagueGroup[] = [];
  loading = false;

  selectedLeague = '';
  selectedPosition = '';
  timeFilter: 'recent' | 'overall' = 'recent';

  positions = ['', 'GK', 'DEF', 'MID', 'FWD'];

  get showGroups(): boolean { return this.selectedLeague === ''; }

  constructor(private soccer: SoccerService, private route: ActivatedRoute) {}

  ngOnInit() {
    this.route.queryParams.subscribe(params => {
      if (params['league']) {
        this.selectedLeague = params['league'];
      }
    });
    this.soccer.getLeagues().subscribe(l => { this.leagues = l; });
    this.load();
  }

  load() {
    this.loading = true;
    if (this.selectedLeague) {
      this.soccer.getTopPredictions(this.selectedLeague, this.selectedPosition, false, this.timeFilter)
        .subscribe({
          next: p => {
            this.predictions = [...p].sort((a, b) => b.predicted_score - a.predicted_score);
            this.loading = false;
          },
          error: () => { this.loading = false; }
        });
    } else {
      forkJoin(
        ALL_LEAGUES.map(l => this.soccer.getTopPredictions(l, this.selectedPosition, false, this.timeFilter))
      ).subscribe({
        next: results => {
          this.leagueGroups = ALL_LEAGUES.map((name, i) => ({
            name,
            players: [...results[i]].sort((a, b) => b.predicted_score - a.predicted_score),
          }))
            .filter(g => g.players.length > 0);
          this.loading = false;
        },
        error: () => { this.loading = false; }
      });
    }
  }

  setTimeFilter(f: 'recent' | 'overall') {
    this.timeFilter = f;
    this.load();
  }

  scoreClass(risk: string) {
    // Map risk to color for the plain text score value (no ring/circle styling).
    return risk === 'low' ? 'list-score--high' : risk === 'medium' ? 'list-score--medium' : 'list-score--low';
  }
}
