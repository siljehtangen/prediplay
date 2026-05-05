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
import { TranslateModule, TranslateService } from '@ngx-translate/core';
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
    MatProgressBarModule, MatFormFieldModule, MatSelectModule, TranslateModule],
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

  constructor(private soccer: SoccerService, private translate: TranslateService) {}

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

    const form = { icon: 'trending_up', text: this.translate.instant('common.formValue', { score: fmt1(player.form_score) }) };

    if (player.position === 'GK') {
      return [
        form,
        { icon: 'shield', text: this.translate.instant('hiddenGems.why.gkShield', { saves: fmtInt(player.saves), conceded: fmtInt(player.goals_conceded) }) },
        { icon: 'swap_horiz', text: this.translate.instant('hiddenGems.why.gkPass', { acc: ratio(player.accurate_passes, player.total_passes) }) },
      ];
    }

    if (player.position === 'DEF') {
      return [
        form,
        { icon: 'shield', text: this.translate.instant('hiddenGems.why.defShield', { duels: ratio(player.duels_won, player.duels_total), tackles: ratio(player.tackles_won, player.tackles_total) }) },
        { icon: 'gps_fixed', text: this.translate.instant('hiddenGems.why.defXA', { xA: fmt1(player.xA) }) },
      ];
    }

    if (player.position === 'MID') {
      return [
        form,
        { icon: 'gps_fixed', text: this.translate.instant('hiddenGems.why.midXGXA', { xG: fmt1(player.xG), xA: fmt1(player.xA) }) },
        { icon: 'key', text: this.translate.instant('hiddenGems.why.midKey', { kp: fmtInt(player.key_passes) }) },
      ];
    }

    return [
      form,
      { icon: 'gps_fixed', text: this.translate.instant('hiddenGems.why.fwdXGXA', { xG: fmt1(player.xG), xA: fmt1(player.xA) }) },
      { icon: 'sports_soccer', text: this.translate.instant('hiddenGems.why.fwdGA', { goals: fmtInt(player.goals), assists: fmtInt(player.assists) }) },
    ];
  }
}
