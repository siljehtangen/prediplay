import { Component } from '@angular/core';
import { RouterOutlet, RouterLink, RouterLinkActive } from '@angular/router';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatListModule } from '@angular/material/list';
import { MatIconModule } from '@angular/material/icon';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { TranslateModule } from '@ngx-translate/core';
import { LanguageService } from './services/language.service';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [RouterOutlet, RouterLink, RouterLinkActive, MatSidenavModule, MatListModule,
    MatIconModule, MatToolbarModule, MatButtonToggleModule, TranslateModule],
  templateUrl: './app.component.html',
  styleUrl: './app.component.scss'
})
export class AppComponent {
  navItems = [
    { path: '/',             icon: 'dashboard',                        key: 'nav.dashboard'    },
    { path: '/top-players',  icon: 'leaderboard',                      key: 'nav.topPlayers'   },
    { path: '/hidden-gems',  icon: 'auto_awesome',                     key: 'nav.hiddenGems'   },
    { path: '/red-flags',    icon: 'flag',                             key: 'nav.redFlags'     },
    { path: '/benchwarmers', icon: 'airline_seat_recline_normal',       key: 'nav.benchwarmers' },
    { path: '/synergy',      icon: 'group_work',                       key: 'nav.synergy'      },
    { path: '/how-it-works', icon: 'info',                             key: 'nav.howItWorks'   },
  ];

  constructor(public lang: LanguageService) {}

  setLang(l: string) { this.lang.use(l); }
}
