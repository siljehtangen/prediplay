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
import { ALL_LEAGUES, League, Player, RedFlagPlayer } from '../../models';

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
        next: p => { this.players = this.sortByRedFlag(p); this.loading = false; },
        error: () => { this.loading = false; }
      });
    } else {
      forkJoin(
        ALL_LEAGUES.map(l => this.soccer.getRedFlags(l, this.selectedPosition, this.timeFilter))
      ).subscribe({
        next: results => {
          this.leagueGroups = ALL_LEAGUES.map((name, i) => ({ name, players: this.sortByRedFlag(results[i]) }))
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

  private sortByRedFlag(players: RedFlagPlayer[]): RedFlagPlayer[] {
    return [...players].sort((a, b) => b.red_flag_score - a.red_flag_score);
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
        { label: 'Recent saves', value: fmtInt(player.recent_saves) },
        { label: 'Recent conceded', value: fmtInt(player.recent_goals_conceded) },
        { label: 'Recent SoT', value: fmtInt(player.recent_shots_on_target) },
        { label: 'Acc. passes', value: passAcc(player.recent_accurate_passes, player.recent_total_passes) },
        { label: 'Recent key passes', value: fmtInt(player.recent_key_passes) },
        { label: 'Recent mins', value: fmtInt(player.recent_minutes) },
      ];
    }

    if (pos === 'DEF') {
      return [
        { label: 'Recent duels', value: ratio(player.recent_duels_won, player.recent_duels_total) },
        { label: 'Recent tackles', value: ratio(player.recent_tackles_won, player.recent_tackles_total) },
        { label: 'Recent key passes', value: fmtInt(player.recent_key_passes) },
        { label: 'Recent xA', value: fmt1(player.recent_xa) },
        { label: 'Recent pass acc.', value: passAcc(player.recent_accurate_passes, player.recent_total_passes) },
        { label: 'Recent mins', value: fmtInt(player.recent_minutes) },
      ];
    }

    if (pos === 'MID') {
      return [
        { label: 'Recent key passes', value: fmtInt(player.recent_key_passes) },
        { label: 'Recent pass acc.', value: passAcc(player.recent_accurate_passes, player.recent_total_passes) },
        { label: 'Recent xG', value: fmt1(player.recent_xg) },
        { label: 'Recent xA', value: fmt1(player.recent_xa) },
        { label: 'Recent duels', value: ratio(player.recent_duels_won, player.recent_duels_total) },
        { label: 'Recent mins', value: fmtInt(player.recent_minutes) },
      ];
    }

    return [
      { label: 'Recent goals', value: fmtInt(player.recent_goals) },
      { label: 'Recent assists', value: fmtInt(player.recent_assists) },
      { label: 'Recent xG', value: fmt1(player.recent_xg) },
      { label: 'Recent xA', value: fmt1(player.recent_xa) },
      { label: 'Recent SoT', value: fmtInt(player.recent_shots_on_target) },
      { label: 'Recent mins', value: fmtInt(player.recent_minutes) },
    ];
  }
}
