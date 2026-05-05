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
import { TranslateModule } from '@ngx-translate/core';
import { SoccerService } from '../../services/soccer.service';
import { ALL_LEAGUES, League, Player, PlayerPrediction, scoreClass } from '../../models';

interface LeagueGroup {
  name: string;
  players: PlayerPrediction[];
}

@Component({
  selector: 'app-top-players',
  standalone: true,
  imports: [CommonModule, RouterLink, FormsModule, MatCardModule, MatSelectModule, MatButtonModule,
    MatIconModule, MatProgressBarModule, MatFormFieldModule, TranslateModule],
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

  scoreClass = scoreClass;

  statsFor(player: Player): Array<{ label: string; value: string }> {
    const fmt1 = (v: number | undefined | null) => (v ?? 0).toFixed(1);
    const fmtInt = (v: number | undefined | null) => String(v ?? 0);
    const ratio = (won: number | undefined | null, total: number | undefined | null) => {
      if (!total) return '—';
      return `${won ?? 0}/${total}`;
    };

    if (player.position === 'GK') {
      return [
        { label: 'common.saves', value: fmtInt(player.saves) },
        { label: 'common.conceded', value: fmtInt(player.goals_conceded) },
        { label: 'common.passAcc', value: ratio(player.accurate_passes, player.total_passes) },
        { label: 'common.minutes', value: fmtInt(player.minutes_played) },
      ];
    }

    if (player.position === 'DEF') {
      return [
        { label: 'common.duels', value: ratio(player.duels_won, player.duels_total) },
        { label: 'common.tackles', value: ratio(player.tackles_won, player.tackles_total) },
        { label: 'common.keyPasses', value: fmtInt(player.key_passes) },
        { label: 'common.xA', value: fmt1(player.xA) },
      ];
    }

    if (player.position === 'MID') {
      return [
        { label: 'common.keyPasses', value: fmtInt(player.key_passes) },
        { label: 'common.xG', value: fmt1(player.xG) },
        { label: 'common.xA', value: fmt1(player.xA) },
        { label: 'common.passAcc', value: ratio(player.accurate_passes, player.total_passes) },
      ];
    }

    return [
      { label: 'common.goals', value: fmtInt(player.goals) },
      { label: 'common.assists', value: fmtInt(player.assists) },
      { label: 'common.xG', value: fmt1(player.xG) },
      { label: 'common.xA', value: fmt1(player.xA) },
    ];
  }
}
