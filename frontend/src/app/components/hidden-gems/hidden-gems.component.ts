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
import { ALL_LEAGUES, League, Player, PlayerPrediction, scoreClass } from '../../models';

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
        next: p => {
          this.gems = [...p].sort((a, b) => b.predicted_score - a.predicted_score);
          this.loading = false;
        },
        error: () => { this.loading = false; }
      });
    } else {
      forkJoin(
        ALL_LEAGUES.map(l => this.soccer.getTopPredictions(l, '', true, this.timeFilter))
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

  scoreClass = scoreClass;

  gemWhyFor(player: Player): Array<{ icon: string; text: string }> {
    const fmt1 = (v: number | undefined | null) => (v ?? 0).toFixed(1);
    const fmtInt = (v: number | undefined | null) => String(v ?? 0);
    const ratio = (won: number | undefined | null, total: number | undefined | null) => {
      if (!total) return '—';
      return `${won ?? 0}/${total}`;
    };

    const form = { icon: 'trending_up', text: `Form: ${fmt1(player.form_score)}/10` };

    if (player.position === 'GK') {
      return [
        form,
        { icon: 'shield', text: `Saves: ${fmtInt(player.saves)} / Conceded: ${fmtInt(player.goals_conceded)}` },
        { icon: 'swap_horiz', text: `Pass acc. ${ratio(player.accurate_passes, player.total_passes)} (low ownership)` },
      ];
    }

    if (player.position === 'DEF') {
      return [
        form,
        { icon: 'shield', text: `Duels: ${ratio(player.duels_won, player.duels_total)} · Tackles: ${ratio(player.tackles_won, player.tackles_total)}` },
        { icon: 'gps_fixed', text: `xA: ${fmt1(player.xA)} (low ownership)` },
      ];
    }

    if (player.position === 'MID') {
      return [
        form,
        { icon: 'gps_fixed', text: `xG ${fmt1(player.xG)} / xA ${fmt1(player.xA)}` },
        { icon: 'key', text: `Key passes: ${fmtInt(player.key_passes)} (low ownership)` },
      ];
    }

    // FWD
    return [
      form,
      { icon: 'gps_fixed', text: `xG ${fmt1(player.xG)} / xA ${fmt1(player.xA)}` },
      { icon: 'sports_soccer', text: `${fmtInt(player.goals)}G ${fmtInt(player.assists)}A (low ownership)` },
    ];
  }
}
