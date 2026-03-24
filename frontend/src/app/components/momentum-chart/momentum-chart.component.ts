import { Component, Input, OnChanges, AfterViewInit, ViewChild, ElementRef, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { MatIconModule } from '@angular/material/icon';
import { Chart, LineController, LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip } from 'chart.js';
import { SoccerService } from '../../services/soccer.service';
import { MomentumData } from '../../models';

Chart.register(LineController, LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip);

@Component({
  selector: 'app-momentum-chart',
  standalone: true,
  imports: [CommonModule, MatIconModule],
  templateUrl: './momentum-chart.component.html',
  styleUrl: './momentum-chart.component.scss'
})
export class MomentumChartComponent implements OnChanges, OnDestroy {
  @Input() playerId!: number;
  @ViewChild('chartCanvas') canvasRef!: ElementRef<HTMLCanvasElement>;

  data: MomentumData | null = null;
  loading = false;
  private chart: Chart | null = null;

  constructor(private soccer: SoccerService) {}

  ngOnChanges() {
    if (this.playerId) this.load();
  }

  ngOnDestroy() {
    this.chart?.destroy();
  }

  private load() {
    this.loading = true;
    this.soccer.getMomentum(this.playerId).subscribe({
      next: d => {
        this.data = d;
        this.loading = false;
        setTimeout(() => this.buildChart(d), 50);
      },
      error: () => { this.loading = false; }
    });
  }

  private buildChart(d: MomentumData) {
    if (!this.canvasRef) return;
    const color = d.trend === 'rising' ? '#81c784' : d.trend === 'falling' ? '#ef9a9a' : '#a89cff';
    this.chart?.destroy();
    this.chart = new Chart(this.canvasRef.nativeElement, {
      type: 'line',
      data: {
        labels: d.games.map((_, i) => `GW${i + 1}`),
        datasets: [{
          data: d.games.map(g => g.score),
          borderColor: color,
          backgroundColor: color + '22',
          tension: 0.4,
          fill: true,
          pointBackgroundColor: color,
          pointRadius: 4,
          pointHoverRadius: 6,
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: false },
          tooltip: {
            backgroundColor: '#1a1a2e',
            titleColor: '#e8e8f0',
            bodyColor: '#aaa',
            borderColor: 'rgba(108,99,255,.3)',
            borderWidth: 1,
          }
        },
        scales: {
          x: { grid: { color: 'rgba(255,255,255,.05)' }, ticks: { color: '#666' } },
          y: { min: 0, max: 10, grid: { color: 'rgba(255,255,255,.05)' }, ticks: { color: '#666' } }
        }
      }
    });
  }

  get trendIcon() {
    if (!this.data) return 'remove';
    return this.data.trend === 'rising' ? 'trending_up' : this.data.trend === 'falling' ? 'trending_down' : 'trending_flat';
  }

  get trendClass() {
    return this.data ? `trend--${this.data.trend === 'rising' ? 'up' : this.data.trend === 'falling' ? 'down' : 'stable'}` : '';
  }
}
