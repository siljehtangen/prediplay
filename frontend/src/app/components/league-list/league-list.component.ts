import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router } from '@angular/router';
import { MatCardModule } from '@angular/material/card';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { SoccerService } from '../../services/soccer.service';
import { League } from '../../models';

const COUNTRY_FLAGS: Record<string, string> = {
  england: '🏴󠁧󠁢󠁥󠁮󠁧󠁿', spain: '🇪🇸', germany: '🇩🇪', italy: '🇮🇹',
  france: '🇫🇷', portugal: '🇵🇹', netherlands: '🇳🇱', scotland: '🏴󠁧󠁢󠁳󠁣󠁴󠁿',
  default: '🌍'
};

@Component({
  selector: 'app-league-list',
  standalone: true,
  imports: [CommonModule, MatCardModule, MatButtonModule, MatIconModule],
  templateUrl: './league-list.component.html',
  styleUrl: './league-list.component.scss'
})
export class LeagueListComponent implements OnInit {
  leagues: League[] = [];
  loading = false;

  constructor(private soccer: SoccerService, private router: Router) {}

  ngOnInit() {
    this.loading = true;
    this.soccer.getLeagues().subscribe({
      next: l => { this.leagues = l; this.loading = false; },
      error: () => { this.loading = false; }
    });
  }

  flag(country: string): string {
    const key = country?.toLowerCase() ?? '';
    return COUNTRY_FLAGS[key] ?? COUNTRY_FLAGS['default'];
  }

  viewPlayers(league: League) {
    this.router.navigate(['/top-players'], { queryParams: { league: league.name } });
  }
}
