import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { FormsModule } from '@angular/forms';
import { MatCardModule } from '@angular/material/card';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatSelectModule } from '@angular/material/select';
import { forkJoin } from 'rxjs';
import { SoccerService } from '../../services/soccer.service';
import { League, PlayerPrediction } from '../../models';

const ALL_LEAGUES = ['Premier League', 'La Liga', 'Bundesliga', 'Serie A', 'Ligue 1'];

interface LeagueGroup {
  name: string;
  players: PlayerPrediction[];
}

@Component({
  selector: 'app-hidden-gems',
  standalone: true,
  imports: [CommonModule, RouterLink, FormsModule, MatCardModule, MatButtonModule, MatIconModule,
    MatProgressBarModule, MatFormFieldModule, MatSelectModule],
  templateUrl: './hidden-gems.component.html',
  styleUrl: './hidden-gems.component.scss'
})
export class HiddenGemsComponent implements OnInit {
  leagues: League[] = [];
  gems: PlayerPrediction[] = [];
  leagueGroups: LeagueGroup[] = [];
  loading = false;
  selectedLeague = '';
  timeFilter: 'recent' | 'overall' = 'recent';

  get showGroups(): boolean { return this.selectedLeague === ''; }

  constructor(private soccer: SoccerService) {}

  ngOnInit() {
    this.soccer.getLeagues().subscribe(l => this.leagues = l);
    this.load();
  }

  load() {
    this.loading = true;
    if (this.selectedLeague) {
      this.soccer.getTopPredictions(this.selectedLeague, '', true, this.timeFilter).subscribe({
        next: p => { this.gems = p; this.loading = false; },
        error: () => { this.loading = false; }
      });
    } else {
      forkJoin(
        ALL_LEAGUES.map(l => this.soccer.getTopPredictions(l, '', true, this.timeFilter))
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

  scoreClass(risk: string) {
    return risk === 'low' ? 'score-circle--high' : risk === 'medium' ? 'score-circle--medium' : 'score-circle--low';
  }
}
