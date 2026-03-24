import { Component, Input, Output, EventEmitter } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { MatSliderModule } from '@angular/material/slider';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatCardModule } from '@angular/material/card';
import { PredictionWeights, DEFAULT_WEIGHTS } from '../../models';

@Component({
  selector: 'app-prediction-controls',
  standalone: true,
  imports: [CommonModule, FormsModule, MatSliderModule, MatButtonModule, MatIconModule, MatCardModule],
  templateUrl: './prediction-controls.component.html',
  styleUrl: './prediction-controls.component.scss'
})
export class PredictionControlsComponent {
  @Input() collapsed = false;
  @Output() weightsChange = new EventEmitter<PredictionWeights>();

  weights: PredictionWeights = { ...DEFAULT_WEIGHTS };

  sliders = [
    { key: 'form',      label: 'Form Score',          icon: 'trending_up'  },
    { key: 'threat',    label: 'xG / xA Threat',      icon: 'gps_fixed'    },
    { key: 'opponent',  label: 'Opponent Difficulty',  icon: 'shield'       },
    { key: 'minutes',   label: 'Minutes Played',       icon: 'timer'        },
    { key: 'home_away', label: 'Home / Away Factor',   icon: 'home'         },
  ] as const;

  onSliderChange() {
    this.normalize();
    this.emit();
  }

  reset() {
    this.weights = { ...DEFAULT_WEIGHTS };
    this.emit();
  }

  get(key: string): number {
    return (this.weights as any)[key];
  }

  set(key: string, value: number) {
    (this.weights as any)[key] = value / 100;
    this.onSliderChange();
  }

  pct(key: string): number {
    return Math.round(this.get(key) * 100);
  }

  private normalize() {
    const sum = Object.values(this.weights).reduce((a, b) => a + b, 0);
    if (sum === 0) { this.weights = { ...DEFAULT_WEIGHTS }; return; }
    (Object.keys(this.weights) as (keyof PredictionWeights)[]).forEach(k => {
      this.weights[k] = Math.round((this.weights[k] / sum) * 1000) / 1000;
    });
  }

  private emit() {
    this.weightsChange.emit({ ...this.weights });
  }
}
