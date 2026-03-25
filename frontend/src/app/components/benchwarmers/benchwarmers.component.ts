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
import { BenchwarmerPlayer, League } from '../../models';

const ALL_LEAGUES = ['Premier League', 'La Liga', 'Bundesliga', 'Serie A', 'Ligue 1'];

interface LeagueGroup {
  name: string;
  players: BenchwarmerPlayer[];
}

@Component({
  selector: 'app-benchwarmers',
  standalone: true,
  imports: [CommonModule, RouterLink, FormsModule, MatCardModule, MatButtonModule, MatIconModule,
    MatProgressBarModule, MatFormFieldModule, MatSelectModule],
  templateUrl: './benchwarmers.component.html',
  styleUrl: './benchwarmers.component.scss'
})
export class BenchwarmersComponent implements OnInit {
  leagues: League[] = [];
  players: BenchwarmerPlayer[] = [];
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
      this.soccer.getBenchwarmers(this.selectedLeague, this.selectedPosition, this.timeFilter).subscribe({
        next: p => { this.players = p; this.loading = false; },
        error: () => { this.loading = false; }
      });
    } else {
      forkJoin(
        ALL_LEAGUES.map(l => this.soccer.getBenchwarmers(l, this.selectedPosition, this.timeFilter))
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

  labelIcon(label: string): string {
    if (label === 'Rock Solid') return 'verified';
    if (label === 'Steady Option') return 'trending_flat';
    return 'swap_horiz';
  }

  labelClass(label: string): string {
    if (label === 'Rock Solid') return 'label--solid';
    if (label === 'Steady Option') return 'label--steady';
    return 'label--rotation';
  }
}
