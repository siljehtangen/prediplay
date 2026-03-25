import { Routes } from '@angular/router';

export const routes: Routes = [
  { path: '', loadComponent: () => import('./components/dashboard/dashboard.component').then(m => m.DashboardComponent) },
  { path: 'top-players', loadComponent: () => import('./components/top-players/top-players.component').then(m => m.TopPlayersComponent) },
  { path: 'hidden-gems', loadComponent: () => import('./components/hidden-gems/hidden-gems.component').then(m => m.HiddenGemsComponent) },
  { path: 'red-flags', loadComponent: () => import('./components/red-flags/red-flags.component').then(m => m.RedFlagsComponent) },
  { path: 'benchwarmers', loadComponent: () => import('./components/benchwarmers/benchwarmers.component').then(m => m.BenchwarmersComponent) },
  { path: 'synergy', loadComponent: () => import('./components/synergy/synergy.component').then(m => m.SynergyComponent) },
  { path: 'events', redirectTo: '' },
  { path: 'player/:id', loadComponent: () => import('./components/player-detail/player-detail.component').then(m => m.PlayerDetailComponent) },
  { path: 'how-it-works', loadComponent: () => import('./components/info/info.component').then(m => m.InfoComponent) },
  { path: '**', redirectTo: '' },
];
