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
import { League, RedFlagPlayer } from '../../models';

const ALL_LEAGUES = ['Premier League', 'La Liga', 'Bundesliga', 'Serie A', 'Ligue 1'];

interface LeagueGroup {
  name: string;
  players: RedFlagPlayer[];
}

@Component({
  selector: 'app-red-flags',
  standalone: true,
  imports: [CommonModule, RouterLink, FormsModule, MatCardModule, MatButtonModule, MatIconModule,
    MatProgressBarModule, MatFormFieldModule, MatSelectModule],
  templateUrl: './red-flags.component.html',
  styleUrl: './red-flags.component.scss'
})
export class RedFlagsComponent implements OnInit {
  leagues: League[] = [];
  players: RedFlagPlayer[] = [];
  leagueGroups: LeagueGroup[] = [];
  loading = false;
  selectedLeague = '';
  selectedPosition = '';
  timeFilter: 'recent' | 'overall' = 'recent';

  positions = ['GK', 'DEF', 'MID', 'FWD'];

  get showGroups(): boolean { return this.selectedLeague === ''; }

  constructor(private soccer: SoccerService) {}

  ngOnInit() {
    this.soccer.getLeagues().subscribe(l => this.leagues = l);
    this.load();
  }

  load() {
    this.loading = true;
    if (this.selectedLeague) {
      this.soccer.getRedFlags(this.selectedLeague, this.selectedPosition, this.timeFilter).subscribe({
        next: p => { this.players = p; this.loading = false; },
        error: () => { this.loading = false; }
      });
    } else {
      forkJoin(
        ALL_LEAGUES.map(l => this.soccer.getRedFlags(l, this.selectedPosition, this.timeFilter))
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

  severityClass(score: number): string {
    if (score >= 7.5) return 'severity--critical';
    if (score >= 5.5) return 'severity--high';
    return 'severity--medium';
  }

  severityLabel(score: number): string {
    if (score >= 7.5) return 'Critical';
    if (score >= 5.5) return 'High';
    return 'Medium';
  }
}
