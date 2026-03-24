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
import { PredictionControlsComponent } from '../prediction-controls/prediction-controls.component';
import { League, PlayerPrediction, PredictionWeights, DEFAULT_WEIGHTS } from '../../models';

const ALL_LEAGUES = ['Premier League', 'La Liga', 'Bundesliga', 'Serie A', 'Ligue 1'];

interface LeagueGroup {
  name: string;
  players: PlayerPrediction[];
}

@Component({
  selector: 'app-top-players',
  standalone: true,
  imports: [CommonModule, RouterLink, FormsModule, MatCardModule, MatSelectModule, MatButtonModule,
    MatIconModule, MatProgressBarModule, MatFormFieldModule, PredictionControlsComponent],
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
  weights: PredictionWeights = { ...DEFAULT_WEIGHTS };

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
      this.soccer.getTopPredictions(this.selectedLeague, this.selectedPosition, false, this.weights, this.timeFilter)
        .subscribe({
          next: p => { this.predictions = p; this.loading = false; },
          error: () => { this.loading = false; }
        });
    } else {
      forkJoin(
        ALL_LEAGUES.map(l => this.soccer.getTopPredictions(l, this.selectedPosition, false, this.weights, this.timeFilter))
      ).subscribe({
        next: results => {
          this.leagueGroups = ALL_LEAGUES.map((name, i) => ({ name, players: results[i] }))
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

  onWeightsChange(w: PredictionWeights) {
    this.weights = w;
    this.load();
  }

  scoreClass(risk: string) {
    return risk === 'low' ? 'score-circle--high' : risk === 'medium' ? 'score-circle--medium' : 'score-circle--low';
  }
}
