import { Component, OnInit, ViewChild, ElementRef, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { MatCardModule } from '@angular/material/card';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatSelectModule } from '@angular/material/select';
import { MatFormFieldModule } from '@angular/material/form-field';
import { Chart, BarController, BarElement, LinearScale, CategoryScale, Tooltip, Legend } from 'chart.js';
import { SoccerService } from '../../services/soccer.service';
import { Player, League, SynergyResult } from '../../models';

Chart.register(BarController, BarElement, LinearScale, CategoryScale, Tooltip, Legend);

@Component({
  selector: 'app-synergy',
  standalone: true,
  imports: [CommonModule, FormsModule, MatCardModule, MatButtonModule, MatIconModule,
    MatSelectModule, MatFormFieldModule],
  templateUrl: './synergy.component.html',
  styleUrl: './synergy.component.scss'
})
export class SynergyComponent implements OnInit, OnDestroy {
  @ViewChild('synergyCanvas') canvasRef!: ElementRef<HTMLCanvasElement>;

  leagues: League[] = [];
  selectedLeague = '';
  allPlayers: Player[] = [];
  selectedIds: number[] = [];
  result: SynergyResult | null = null;
  loadingLeagues = false;
  loading = false;
  analyzing = false;
  private chart: Chart | null = null;

  constructor(private soccer: SoccerService) {}

  ngOnInit() {
    this.loadingLeagues = true;
    this.soccer.getLeagues().subscribe({
      next: l => { this.leagues = l; this.loadingLeagues = false; },
      error: () => { this.loadingLeagues = false; }
    });
  }

  onLeagueChange() {
    this.selectedIds = [];
    this.result = null;
    this.chart?.destroy();
    this.chart = null;
    if (!this.selectedLeague) { this.allPlayers = []; return; }
    this.loading = true;
    this.soccer.getPlayers(this.selectedLeague).subscribe({
      next: p => { this.allPlayers = p; this.loading = false; },
      error: () => { this.loading = false; }
    });
  }

  ngOnDestroy() { this.chart?.destroy(); }

  analyze() {
    if (this.selectedIds.length < 2) return;
    this.analyzing = true;
    this.soccer.getSynergy(this.selectedIds).subscribe({
      next: r => {
        this.result = r;
        this.analyzing = false;
        setTimeout(() => this.buildChart(r), 50);
      },
      error: () => { this.analyzing = false; }
    });
  }

  getPlayer(id: number): Player | undefined {
    return this.allPlayers.find(p => p.id === id);
  }

  private buildChart(r: SynergyResult) {
    if (!this.canvasRef) return;
    this.chart?.destroy();
    const names = r.players.map(p => p.name.split(' ').pop() ?? p.name);
    this.chart = new Chart(this.canvasRef.nativeElement, {
      type: 'bar',
      data: {
        labels: [...names, 'Combined', 'w/ Synergy'],
        datasets: [{
          label: 'Score',
          data: [
            ...r.players.map(() => +(r.total_predicted / r.players.length).toFixed(1)),
            +r.total_predicted.toFixed(1),
            +r.synergy_score.toFixed(1)
          ],
          backgroundColor: [...r.players.map(() => '#6c63ff'), '#81c784', '#ffd54f'],
          borderRadius: 4,
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: false },
          tooltip: { backgroundColor: '#1a1a2e', titleColor: '#e8e8f0', bodyColor: '#aaa' }
        },
        scales: {
          x: { grid: { color: 'rgba(255,255,255,.05)' }, ticks: { color: '#aaa' } },
          y: { grid: { color: 'rgba(255,255,255,.05)' }, ticks: { color: '#666' } }
        }
      }
    });
  }
}
