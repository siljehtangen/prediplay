import { Injectable } from '@angular/core';
import { TranslateService } from '@ngx-translate/core';

@Injectable({ providedIn: 'root' })
export class LanguageService {
  private readonly KEY = 'prediplay_lang';

  constructor(private translate: TranslateService) {
    translate.addLangs(['en', 'nb']);
    translate.setDefaultLang('en');
    const saved = localStorage.getItem(this.KEY);
    const lang = saved ?? this.detectLang();
    translate.use(lang);
  }

  private detectLang(): string {
    const b = navigator.language.toLowerCase();
    return (b.startsWith('nb') || b.startsWith('no') || b.startsWith('nn')) ? 'nb' : 'en';
  }

  use(lang: string) {
    this.translate.use(lang);
    localStorage.setItem(this.KEY, lang);
  }

  get current(): string { return this.translate.currentLang || 'en'; }
}
