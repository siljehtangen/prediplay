import { Component, OnInit, ViewChild, ElementRef, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { MatCardModule } from '@angular/material/card';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatTableModule } from '@angular/material/table';
import { MatChipsModule } from '@angular/material/chips';
import { forkJoin, of, Subscription } from 'rxjs';
import { catchError, timeout } from 'rxjs/operators';
import { Chart, BarController, BarElement, LinearScale, CategoryScale, Tooltip } from 'chart.js';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { SoccerService } from '../../services/soccer.service';
import { MomentumChartComponent } from '../momentum-chart/momentum-chart.component';
import { PlayerPrediction, PlayerStat } from '../../models';

Chart.register(BarController, BarElement, LinearScale, CategoryScale, Tooltip);

@Component({
  selector: 'app-player-detail',
  standalone: true,
  imports: [CommonModule, RouterLink, MatCardModule, MatButtonModule, MatIconModule,
    MatProgressBarModule, MatTableModule, MatChipsModule,
    MomentumChartComponent, TranslateModule],
  templateUrl: './player-detail.component.html',
  styleUrl: './player-detail.component.scss'
})
export class PlayerDetailComponent implements OnInit, OnDestroy {
  @ViewChild('breakdownCanvas') canvasRef!: ElementRef<HTMLCanvasElement>;

  playerId!: number;
  prediction: PlayerPrediction | null = null;
  stats: PlayerStat[] = [];
  photoUrl = '';
  photoError = false;
  loading = false;
  private chart: Chart | null = null;
  private paramSub?: Subscription;

  statColumns = ['date', 'opponent', 'score', 'mins', 'rating', 'goals', 'assists', 'xG', 'xA', 'shots', 'key_pass'];

  constructor(private route: ActivatedRoute, private soccer: SoccerService, private translate: TranslateService) {}

  ngOnInit() {
    this.paramSub = this.route.paramMap.subscribe(params => {
      this.playerId = Number(params.get('id'));
      this.photoUrl = this.soccer.getPlayerPhotoUrl(this.playerId);
      this.photoError = false;
      this.load();
    });
  }

  ngOnDestroy() {
    this.chart?.destroy();
    this.paramSub?.unsubscribe();
  }

  load() {
    this.loading = true;
    forkJoin({
      prediction: this.soccer.getPlayerPrediction(this.playerId).pipe(
        timeout(20000),
        catchError(() => of(null))
      ),
      stats: this.soccer.getPlayerStats(this.playerId).pipe(
        timeout(20000),
        catchError(() => of([] as any[]))
      ),
    }).subscribe({
      next: ({ prediction, stats }) => {
        this.prediction = prediction;
        this.stats = (stats as any[]).slice(0, 10);
        this.loading = false;
        if (prediction) setTimeout(() => this.buildChart(prediction), 50);
      },
      error: () => { this.loading = false; }
    });
  }

  scoreClass(risk: string) {
    return risk === 'low' ? 'score-circle--high' : risk === 'medium' ? 'score-circle--medium' : 'score-circle--low';
  }

  age(dob: string): number {
    if (!dob) return 0;
    const birth = new Date(dob);
    const today = new Date();
    let age = today.getFullYear() - birth.getFullYear();
    const m = today.getMonth() - birth.getMonth();
    if (m < 0 || (m === 0 && today.getDate() < birth.getDate())) age--;
    return age;
  }

  marketValueM(v: number): string {
    if (!v) return '—';
    return `€${(v / 1_000_000).toFixed(0)}M`;
  }

  opponent(stat: PlayerStat, teamName: string): string {
    return stat.event.home_team.toLowerCase() === teamName.toLowerCase()
      ? stat.event.away_team
      : stat.event.home_team;
  }

  matchScore(stat: PlayerStat): string {
    return `${stat.event.home_score}–${stat.event.away_score}`;
  }

  formatDate(d: string): string {
    const locale = this.translate.currentLang === 'nb' ? 'nb-NO' : 'en-GB';
    return new Date(d).toLocaleDateString(locale, { day: 'numeric', month: 'short' });
  }

  private buildChart(p: PlayerPrediction) {
    if (!this.canvasRef) return;
    this.chart?.destroy();
    this.chart = new Chart(this.canvasRef.nativeElement, {
      type: 'bar',
      data: {
        labels: [
          this.translate.instant('common.form'),
          this.translate.instant('common.xG') + '/' + this.translate.instant('common.xA'),
          this.translate.instant('info.playerScore.opponent.pill'),
          this.translate.instant('common.minutes'),
          this.translate.instant('info.playerScore.defensive.pill'),
        ],
        datasets: [{
          data: [p.form_contribution, p.threat_contribution, p.opponent_difficulty, p.minutes_likelihood, p.defensive_contribution],
          backgroundColor: ['#6c63ff', '#81c784', '#ffd54f', '#64b5f6', '#f06292'],
          borderRadius: 4,
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        indexAxis: 'y',
        plugins: {
          legend: { display: false },
          tooltip: { backgroundColor: '#1a1a2e', titleColor: '#e8e8f0', bodyColor: '#aaa' }
        },
        scales: {
          // Contributions are 0–10, so the x-axis must scale accordingly.
          x: { grid: { color: 'rgba(255,255,255,.05)' }, ticks: { color: '#666' }, min: 0, max: 10, },
          y: { grid: { display: false }, ticks: { color: '#aaa' } }
        }
      }
    });
  }
}
