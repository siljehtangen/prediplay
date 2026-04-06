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
import { ALL_LEAGUES, BenchwarmerPlayer, League, Player } from '../../models';

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
        next: p => { this.players = this.sortByConsistency(p); this.loading = false; },
        error: () => { this.loading = false; }
      });
    } else {
      forkJoin(
        ALL_LEAGUES.map(l => this.soccer.getBenchwarmers(l, this.selectedPosition, this.timeFilter))
      ).subscribe({
        next: results => {
          this.leagueGroups = ALL_LEAGUES.map((name, i) => ({ name, players: this.sortByConsistency(results[i]) }))
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

  private sortByConsistency(players: BenchwarmerPlayer[]): BenchwarmerPlayer[] {
    return [...players].sort((a, b) => b.consistency_score - a.consistency_score);
  }

  statsFor(player: Player): Array<{ label: string; value: string }> {
    const pos = player.position;
    const fmt1 = (v: number | undefined | null) => (v ?? 0).toFixed(1);
    const fmtInt = (v: number | undefined | null) => (v ?? 0).toString();
    const ratio = (won: number | undefined | null, total: number | undefined | null) => {
      if (!total || (total as number) === 0) return '—';
      return `${won ?? 0}/${total}`;
    };
    const passAcc = (accurate: number | undefined | null, total: number | undefined | null) => {
      if (!total || (total as number) === 0) return '—';
      return `${accurate ?? 0}/${total}`;
    };

    if (pos === 'GK') {
      return [
        { label: 'Form', value: fmt1(player.form_score) },
        { label: 'Saves', value: fmtInt(player.saves) },
        { label: 'Conceded', value: fmtInt(player.goals_conceded) },
        { label: 'Acc. passes', value: passAcc(player.accurate_passes, player.total_passes) },
        { label: 'Key passes', value: fmtInt(player.key_passes) },
        { label: 'Mins', value: fmtInt(player.minutes_played) },
      ];
    }

    if (pos === 'DEF') {
      return [
        { label: 'Form', value: fmt1(player.form_score) },
        { label: 'Duels', value: ratio(player.duels_won, player.duels_total) },
        { label: 'Tackles', value: ratio(player.tackles_won, player.tackles_total) },
        { label: 'Key passes', value: fmtInt(player.key_passes) },
        { label: 'xA', value: fmt1(player.xA) },
        { label: 'Mins', value: fmtInt(player.minutes_played) },
      ];
    }

    if (pos === 'MID') {
      return [
        { label: 'Form', value: fmt1(player.form_score) },
        { label: 'Key passes', value: fmtInt(player.key_passes) },
        { label: 'Pass acc.', value: passAcc(player.accurate_passes, player.total_passes) },
        { label: 'xG', value: fmt1(player.xG) },
        { label: 'xA', value: fmt1(player.xA) },
        { label: 'Mins', value: fmtInt(player.minutes_played) },
      ];
    }

    // FWD fallback
    return [
      { label: 'Form', value: fmt1(player.form_score) },
      { label: 'Goals', value: fmtInt(player.goals) },
      { label: 'Assists', value: fmtInt(player.assists) },
      { label: 'xG', value: fmt1(player.xG) },
      { label: 'xA', value: fmt1(player.xA) },
      { label: 'Mins', value: fmtInt(player.minutes_played) },
    ];
  }
}
