import { Component } from '@angular/core';
import { RouterOutlet, RouterLink, RouterLinkActive } from '@angular/router';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatListModule } from '@angular/material/list';
import { MatIconModule } from '@angular/material/icon';
import { MatToolbarModule } from '@angular/material/toolbar';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [RouterOutlet, RouterLink, RouterLinkActive, MatSidenavModule, MatListModule, MatIconModule, MatToolbarModule],
  templateUrl: './app.component.html',
  styleUrl: './app.component.scss'
})
export class AppComponent {
  navItems = [
    { path: '/',            icon: 'dashboard',       label: 'Dashboard'    },
    { path: '/top-players', icon: 'leaderboard',     label: 'Top Players'  },
    { path: '/hidden-gems',  icon: 'auto_awesome',    label: 'Hidden Gems'  },
    { path: '/red-flags',    icon: 'flag',            label: 'Red Flags'    },
    { path: '/benchwarmers', icon: 'airline_seat_recline_normal', label: 'Benchwarmers' },
    { path: '/synergy',     icon: 'group_work',      label: 'Synergy'      },
    { path: '/how-it-works',  icon: 'info',            label: 'How It Works' },
  ];
}
