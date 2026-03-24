import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { MatCardModule } from '@angular/material/card';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatChipsModule } from '@angular/material/chips';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MatNativeDateModule } from '@angular/material/core';
import { SoccerService } from '../../services/soccer.service';
import { Event, Prediction, League } from '../../models';

@Component({
  selector: 'app-event-list',
  standalone: true,
  imports: [CommonModule, FormsModule, MatCardModule, MatButtonModule, MatIconModule,
    MatFormFieldModule, MatInputModule, MatSelectModule, MatChipsModule,
    MatDatepickerModule, MatNativeDateModule],
  templateUrl: './event-list.component.html',
  styleUrl: './event-list.component.scss'
})
export class EventListComponent implements OnInit {
  events: Event[] = [];
  predictions: Prediction[] = [];
  leagues: League[] = [];
  liveEvents: Event[] = [];
  loading = false;

  dateFrom: Date | null = null;
  dateTo: Date | null = null;
  selectedLeague = '';

  constructor(private soccer: SoccerService) {}

  ngOnInit() {
    const today = new Date();
    this.dateFrom = today;
    this.dateTo = new Date(today.getFullYear(), today.getMonth(), today.getDate() + 7);
    this.soccer.getLeagues().subscribe(l => this.leagues = l);
    this.soccer.getLive().subscribe(live => this.liveEvents = live);
    this.load();
  }

  private toApiDate(d: Date | null): string {
    return d ? d.toISOString().split('T')[0] : '';
  }

  load() {
    this.loading = true;
    this.soccer.getEvents(this.toApiDate(this.dateFrom), this.toApiDate(this.dateTo), this.selectedLeague).subscribe({
      next: e => { this.events = e; this.loading = false; },
      error: () => { this.loading = false; }
    });
    this.soccer.getPredictions(true).subscribe(p => this.predictions = p);
  }

  getPrediction(homeTeam: string | undefined, awayTeam: string | undefined): Prediction | undefined {
    if (!homeTeam || !awayTeam) return undefined;
    return this.predictions.find(p =>
      p.home_team?.toLowerCase() === homeTeam.toLowerCase() &&
      p.away_team?.toLowerCase() === awayTeam.toLowerCase()
    );
  }

  formatDate(d: string): string {
    return new Date(d).toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
  }

  isLive(eventId: number): boolean {
    return this.liveEvents.some(e => e.id === eventId);
  }
}
