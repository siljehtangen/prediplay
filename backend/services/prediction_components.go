package services

import (
	"math"
	"prediplay/backend/models"
	"time"
)

// Every component is calibrated so that a genuinely elite player scores ~8-10,
// a solid average player scores ~5-6, and a poor player scores ~2-3.
// This spread prevents scores from clustering in the 5.5-6.5 band.

// formComponent blends season and recent match ratings for a forward-looking form signal.
// Recent form is weighted more heavily since it better predicts the upcoming match.
// GKs get a 50/50 blend because a string of recent clean sheets (or blunders) matters more
// game-to-game for a keeper than for outfield players.
func formComponent(p models.Player) float64 {
	seasonForm := p.FormScore
	if seasonForm <= 0 {
		seasonForm = 6.0
	}
	if p.RecentFormScore > 0 && p.RecentGamesPlayed >= 2 {
		if p.Position == "GK" {
			// GK: 50/50 — recent clean sheets / howlers are strongly predictive
			return math.Max(0, math.Min(10, seasonForm*0.50+p.RecentFormScore*0.50))
		}
		// Outfield: 60% season stability + 40% recent form
		return math.Max(0, math.Min(10, seasonForm*0.60+p.RecentFormScore*0.40))
	}
	return math.Max(0, math.Min(10, seasonForm))
}

// attackComponent measures goal-scoring threat with three independent signals.
// Recent stats are blended in (40% weight) to capture current offensive momentum.
//
// xG scaling is position-aware so the cap reflects realistic xG ranges per role:
//   - FWD: cap 10 (multiplier ×14) — elite strikers (0.70+ xG/90) reach the cap;
//     0.40→5.6, 0.57→8.0, 0.71→10.0. Haaland and a squad striker are now distinct.
//   - MID: cap 8  (multiplier ×14) — 0.57 xG/90 is exceptional for a MID.
//   - DEF: cap 5  (multiplier ×10) — DEF goals are rare; even 0.50 xG/90 is elite.
//   - GK:  cap 3  (multiplier ×6)  — attack contribution is negligible; weight=0 in calcPrediction.
//
// Conversion bonus (+0-2): rewards clinical finishers without double-counting xG.
// Shot volume (+0-2):      how often the player tests the keeper regardless of xG quality.
func attackComponent(p models.Player) float64 {
	mins90 := math.Max(1, float64(p.MinutesPlayed)/90.0)
	xgPer90 := p.XG / mins90
	goalsPer90 := float64(p.Goals) / mins90
	sotPer90 := float64(p.ShotsOnTarget) / mins90

	// Blend in recent stats when available — captures current offensive momentum
	if p.RecentGamesPlayed >= 2 && p.RecentMinutes > 0 {
		recentMins90 := math.Max(1, float64(p.RecentMinutes)/90.0)
		xgPer90 = xgPer90*0.60 + (p.RecentXG/recentMins90)*0.40
		goalsPer90 = goalsPer90*0.60 + (float64(p.RecentGoals)/recentMins90)*0.40
		sotPer90 = sotPer90*0.60 + (float64(p.RecentShotsOnTarget)/recentMins90)*0.40
	}

	// Position-specific xG ceiling — prevents FWDs from plateauing at the same
	// score as midfielders just because the old cap was set for outfield averages.
	var xgMult, xgCap float64
	switch p.Position {
	case "FWD":
		xgMult, xgCap = 14.0, 10.0 // 0.57→8.0, 0.71→10 (Haaland range)
	case "DEF":
		xgMult, xgCap = 10.0, 5.0 // DEF scoring is rare; cap reflects that
	case "GK":
		xgMult, xgCap = 6.0, 3.0 // practically irrelevant (weight 0 in formula)
	default: // MID
		xgMult, xgCap = 14.0, 8.0 // original scaling
	}

	xgScore := math.Min(xgCap, xgPer90*xgMult)
	conversionBonus := math.Min(2, math.Max(0, goalsPer90-xgPer90)*4)
	shotVolume := math.Min(2, sotPer90*0.6)

	return math.Min(10, xgScore+conversionBonus+shotVolume)
}

// creativityComponent is fully position-specific — each position uses the stats
// that are genuinely meaningful for that role.
//
// DEF (build-up quality, not chance creation):
//
//	xA/assists are near-zero for most defenders, so using them as the primary
//	signal collapses all DEFs into the same 0-2 range. Instead:
//	- Key passes per 90 (0-8): 0.5/90→3, 1.0/90→6, 1.5/90→9 — elite attacking FB.
//	- Assist delivery bonus (0-2): DEFs whose key passes actually produce assists
//	  confirm that their crosses and through balls are genuinely dangerous.
//	- Recent blend (60/40): captures attacking FB hot/cold streaks.
//
// MID / FWD (chance creation):
//  1. xA per 90 — quality of chances created (primary predictive signal).
//     Blended 60% season / 40% recent. Elite playmaker ≈ 0.35 xA/90 → ~5.6.
//  2. Assist delivery bonus — assists/90 above xA/90 (team-mates converting well).
//  3. Key pass quality — xA per key pass (dangerous passes, not speculative ones).
//  4. Pass accuracy (MID only, 0-2): 72%→0, 80%→0.9, 88%→2.0.
//     Differentiates tidy ball-players from ball-losers in the build-up.
func creativityComponent(p models.Player) float64 {
	mins90 := math.Max(1, float64(p.MinutesPlayed)/90.0)

	// ── DEF: build-up quality ────────────────────────────────────────────────
	// Attacking full-backs (1.5 KP/90) score ~9; typical CBs (0.3 KP/90) score ~2.
	// This gives the creativity weight real range across the DEF pool, so ball-playing
	// CBs and attacking FBs are correctly differentiated from limited defenders.
	if p.Position == "DEF" {
		kpPer90 := float64(p.KeyPasses) / mins90
		if p.RecentGamesPlayed >= 2 && p.RecentMinutes > 0 {
			recentMins90 := math.Max(1, float64(p.RecentMinutes)/90.0)
			kpPer90 = kpPer90*0.60 + (float64(p.RecentKeyPasses)/recentMins90)*0.40
		}
		kpScore := math.Min(8, kpPer90*6) // 1.33 KP/90 → 8.0 (attacking FB ceiling)
		assistBonus := 0.0
		if p.KeyPasses > 0 {
			// Rewards DEFs whose key passes actually convert to assists
			assistBonus = math.Min(2, float64(p.Assists)/float64(p.KeyPasses)*8)
		}
		return math.Min(10, kpScore+assistBonus)
	}

	// ── MID / FWD: chance creation ───────────────────────────────────────────
	xaPer90 := p.XA / mins90
	assistsPer90 := float64(p.Assists) / mins90

	if p.RecentGamesPlayed >= 2 && p.RecentMinutes > 0 {
		recentMins90 := math.Max(1, float64(p.RecentMinutes)/90.0)
		xaPer90 = xaPer90*0.60 + (p.RecentXA/recentMins90)*0.40
		assistsPer90 = assistsPer90*0.60 + (float64(p.RecentAssists)/recentMins90)*0.40
	}

	xaScore := math.Min(8, xaPer90*16)
	assistBonus := math.Min(2, math.Max(0, assistsPer90-xaPer90)*4)
	kpQuality := 0.0
	if p.KeyPasses > 0 {
		xaPerKP := p.XA / float64(p.KeyPasses)
		kpQuality = math.Min(2, xaPerKP*20) // 0.10 xA/KP → 2.0
	}

	// Pass accuracy bonus — MID only; season totals used for statistical stability.
	// 72%→0, 80%→0.9, 88%→2.0. Differentiates tidy ball-players from ball-losers.
	passAccBonus := 0.0
	if p.Position == "MID" && p.TotalPasses >= 15 {
		passAcc := float64(p.AccuratePasses) / float64(p.TotalPasses)
		passAccBonus = math.Max(0, math.Min(2, (passAcc-0.72)/0.18*2))
	}

	return math.Min(10, xaScore+assistBonus+kpQuality+passAccBonus)
}

// defensiveComponent is fully position-specific.
//
//   - GK:  four signals — save rate (0-7), goals-conceded rate (0-2), recent trend
//     (±1.5), and save volume (0-0.5). The four-signal approach produces a much wider
//     spread than save rate alone, which clusters most keepers in a narrow 65-80%
//     band. Example spread: elite GK (~85% SR, 0.7 GC/g) → ~9-10; average (~72%, 1.2
//     GC/g) → ~6-7; poor (~60%, 1.8 GC/g) → ~2-3.
//     Minimum threshold raised to 2 games and 5 shots faced to avoid single-game noise.
//
//   - DEF: duel win rate (0-4.5) + tackle win rate (0-4.0) + activity volume (0-1.5)
//     + pass accuracy bonus (0-1.5) for ball-playing defenders. The pass accuracy
//     signal differentiates modern ball-playing CBs and attacking full-backs from
//     more limited defenders who can only defend.
//
//   - MID: duel win rate (0-5) + tackle win rate (0-3) + floor of 1.0. Two signals
//     instead of one gives much better spread: a press-heavy DM who wins 65% of
//     duels and 70% of tackles scores ≈8; a creative AM avoiding duels scores ≈4.5.
//
//   - FWD: hold-up play (duel win rate only). Score range 1–6.
func defensiveComponent(p models.Player) float64 {
	mins90 := math.Max(1, float64(p.MinutesPlayed)/90.0)

	switch p.Position {
	case "GK":
		games := math.Max(1, float64(p.GamesPlayed))
		total := float64(p.Saves + p.GoalsConceded)
		// Need at least 2 games and 5 shots faced to have reliable save rate data
		if p.GamesPlayed < 2 || total < 5 {
			return 6.0 // insufficient data — neutral
		}
		saveRate := float64(p.Saves) / total

		// Signal 1: Save rate (0-7) — primary quality signal.
		// 50%→0, 65%→3.5, 72%→5.1, 80%→6.7, 87%→7 (elite).
		// Steeper curve than before to better separate keepers in the 65-82% range.
		rateScore := math.Max(0, math.Min(7, (saveRate-0.50)/0.37*7))

		// Signal 2: Goals conceded per game (0-2) — explicit conceding penalty.
		// Elite (<0.7/game)→2.0; average (1.2/game)→1.1; poor (≥1.8/game)→0.
		goalsConcededPerGame := float64(p.GoalsConceded) / games
		gcScore := math.Max(0, math.Min(2, (1.8-goalsConcededPerGame)/1.1*2))

		// Signal 3: Recent save-rate trend (−1.5 to +1.5).
		// Rewards GKs improving their save rate over the last 3 games; penalises
		// those in poor form. Each 5% swing in save rate = ±0.5.
		trendBonus := 0.0
		recentTotal := float64(p.RecentSaves + p.RecentGoalsConceded)
		if p.RecentGamesPlayed >= 2 && recentTotal >= 3 {
			recentSaveRate := float64(p.RecentSaves) / recentTotal
			trendBonus = math.Max(-1.5, math.Min(1.5, (recentSaveRate-saveRate)*10))
		}

		// Signal 4: Save volume per game (0-0.5) — small bonus for busy, reliable GKs.
		// 3 saves/game → +0.25; 6+ saves/game → +0.5.
		savesPerGame := float64(p.Saves) / games
		volumeBonus := math.Min(0.5, savesPerGame/6.0*0.5)

		// Signal 5: Pass accuracy (0-1.0) — sweeper keeper / ball-playing GK quality.
		// Modern keepers are expected to play out from the back under pressure.
		// Poor distributors are a liability in high-press systems.
		// Requires ≥10 passes logged for statistical reliability.
		// 55%→0, 70%→0.6, 80%+→1.0.
		gkPassAcc := 0.0
		if p.TotalPasses >= 10 {
			passAcc := float64(p.AccuratePasses) / float64(p.TotalPasses)
			gkPassAcc = math.Max(0, math.Min(1.0, (passAcc-0.55)/0.25))
		}

		return math.Max(0, math.Min(10, rateScore+gcScore+trendBonus+volumeBonus+gkPassAcc))

	case "DEF":
		duelRate := 0.50
		if p.DuelsTotal > 0 {
			duelRate = float64(p.DuelsWon) / float64(p.DuelsTotal)
		}
		tackleRate := 0.50
		if p.TacklesTotal > 0 {
			tackleRate = float64(p.TacklesWon) / float64(p.TacklesTotal)
		}
		// Volume bonus: high-activity defenders rewarded (cap at +1.5)
		tacklesPer90 := float64(p.TacklesTotal) / mins90
		activityBonus := math.Min(1.5, tacklesPer90/4.0*1.5)

		// Pass accuracy bonus (0-1.5): ball-playing DEFs who complete passes accurately
		// score higher. Target: 65%→0, 78%→0.78, 90%→1.5.
		// Uses season total for sample stability; requires at least 15 passes logged.
		passAccBonus := 0.0
		if p.TotalPasses >= 15 {
			passAcc := float64(p.AccuratePasses) / float64(p.TotalPasses)
			passAccBonus = math.Max(0, math.Min(1.5, (passAcc-0.65)/0.25*1.5))
		}

		// Quality rates (0-4.5 duel, 0-4.0 tackle) + volume + pass accuracy
		return math.Min(10, duelRate*4.5+tackleRate*4.0+activityBonus+passAccBonus)

	case "MID":
		duelRate := 0.50
		if p.DuelsTotal > 0 {
			duelRate = float64(p.DuelsWon) / float64(p.DuelsTotal)
		}
		tackleRate := 0.50
		if p.TacklesTotal > 0 {
			tackleRate = float64(p.TacklesWon) / float64(p.TacklesTotal)
		}
		// Two signals instead of one: presses + ball-winners get separate credit.
		// DM winning 65% duels + 70% tackles → ~8.1; AM avoiding duels → ~4.5 floor.
		return math.Min(10, duelRate*5.0+tackleRate*3.0+1.0)

	default: // FWD — hold-up play contribution only
		duelRate := 0.50
		if p.DuelsTotal > 0 {
			duelRate = float64(p.DuelsWon) / float64(p.DuelsTotal)
		}
		return math.Min(10, duelRate*5+1.0)
	}
}

// availabilityComponent scores average minutes per game vs a full 90.
// A player averaging 90 min/game scores 10; 45 min/game scores 5.
//
// Blend is 50% season / 50% recent. The previous 70/30 split was too slow to
// detect rotation: a player averaging 88 min/game all season who has been
// subbed at 45 min for 3 straight games still scored ~8.6 instead of signalling
// the rotation risk. At 50/50 that same pattern scores ~7.2 — a meaningful drop.
func availabilityComponent(p models.Player) float64 {
	gamesSeason := math.Max(1, float64(p.GamesPlayed))
	avgMinsSeason := float64(p.MinutesPlayed) / gamesSeason
	availSeason := avgMinsSeason / 90.0 * 10.0

	recentGames := math.Max(1, float64(p.RecentGamesPlayed))
	avgMinsRecent := float64(p.RecentMinutes) / recentGames
	availRecent := avgMinsRecent / 90.0 * 10.0

	avail := availSeason*0.50 + availRecent*0.50
	if avail < 0 {
		avail = 0
	}
	return math.Min(10, avail)
}

// disciplineComponent penalises cards. Starting at 10:
// each yellow card per game costs 5 points; each red costs 15.
//
// Blend is 40% season / 60% recent. Suspension risk is primarily driven by
// recent behaviour — a player picking up 3 yellows in 3 games is imminently
// suspended regardless of how clean they were earlier in the season. The old
// 70/30 blend buried that signal under a clean season record.
func disciplineComponent(p models.Player) float64 {
	gamesSeason := math.Max(1, float64(p.GamesPlayed))
	yellowPerGameSeason := float64(p.YellowCards) / gamesSeason
	redPerGameSeason := float64(p.RedCards) / gamesSeason

	recentGames := math.Max(1, float64(p.RecentGamesPlayed))
	yellowPerGameRecent := float64(p.RecentYellowCards) / recentGames
	redPerGameRecent := float64(p.RecentRedCards) / recentGames

	// 40% season base rate + 60% recent — recent cards dominate (suspension risk)
	yellowPerGame := yellowPerGameSeason*0.40 + yellowPerGameRecent*0.60
	redPerGame := redPerGameSeason*0.40 + redPerGameRecent*0.60

	disc := 10 - yellowPerGame*5 - redPerGame*15
	if disc < 0 {
		disc = 0
	}
	return disc
}

// recentMinutesFactor returns a multiplier (0–1) that penalises players who are
// playing but only receiving cameo minutes in their recent games. The penalty
// is applied the same way as the inactivity penalty — the final score is blended
// toward the below-neutral baseline so a squad player getting 15 min/game cannot
// score positively on their season stats alone.
//
//	≥ 60 min/game: factor=1.00  — starter / near-starter, no penalty
//	45–60 min/game: factor 1.00→0.85 — regular rotation, mild penalty
//	30–45 min/game: factor 0.85→0.60 — fringe player
//	< 30 min/game:  factor 0.40       — cameo role, heavy penalty
//
// Only applied when recent game data exists (RecentGamesPlayed > 0); inactivity
// handles the zero-games case separately.
func recentMinutesFactor(p models.Player) float64 {
	if p.RecentGamesPlayed == 0 {
		return 1.0 // no recent data — inactivity penalty handles this
	}
	avgMins := float64(p.RecentMinutes) / float64(p.RecentGamesPlayed)
	switch {
	case avgMins >= 60:
		return 1.0
	case avgMins >= 45:
		// linear 1.00 → 0.85 over 15 min
		return 1.0 - (60-avgMins)/15*0.15
	case avgMins >= 30:
		// linear 0.85 → 0.60 over 15 min
		return 0.85 - (45-avgMins)/15*0.25
	default:
		// < 30 min/game — trusted only for cameos
		return 0.40
	}
}

// inactivityFactor returns a multiplier (0–1) and a pull-toward baseline (2.5)
// for players who haven't played recently. The longer the absence, the more the
// score is dragged below the neutral midpoint so inactive players never rank high.
//
//	0–14 days:  factor=1.00  → no penalty
//	14–28 days: factor 1.00→0.70 (mild — rotation / rest)
//	28–56 days: factor 0.70→0.35 (significant — injury / dropped)
//	56+ days:   factor=0.35  → score pulled hard toward 2.5
//
// The blended score is: predicted*factor + 2.5*(1-factor).
// A star player (raw 9.0) absent 60 days → 9*0.35 + 2.5*0.65 ≈ 4.8 (below neutral).
// An average player (raw 5.5) absent 60 days → 5.5*0.35 + 2.5*0.65 ≈ 3.5 (clearly negative).
func inactivityFactor(lastMatchDate string) float64 {
	if lastMatchDate == "" {
		return 0.35 // no data — treat as long-term absent
	}
	t, err := time.Parse("2006-01-02", lastMatchDate)
	if err != nil {
		return 0.35
	}
	days := time.Since(t).Hours() / 24
	switch {
	case days <= 14:
		return 1.0
	case days <= 28:
		// linear 1.0 → 0.70 over 14 days
		return 1.0 - (days-14)/14*0.30
	case days <= 56:
		// linear 0.70 → 0.35 over 28 days
		return 0.70 - (days-28)/28*0.35
	default:
		return 0.35
	}
}

// calcPrediction combines seven independent components with position-specific weights.
//
// Each component is now enriched with position-specific signals and recent-form blending
// (see individual component functions for details). Weight rationale per position:
//
//	GK:  Defensive dominates (0.60). The defensive component now has 4 signals
//	     (save rate, goals-conceded rate, recent trend, volume) producing a much
//	     wider score spread than before. Form uses a 50/50 season/recent blend.
//	     Availability and discipline stay low — GKs rarely miss games or get carded.
//	     Weights: form=0.22, def=0.60, avail=0.06, disc=0.04, opp=0.08  → sum 1.00
//
//	DEF: Defensive work is primary (0.30). Now includes pass accuracy signal so
//	     ball-playing CBs and attacking full-backs score distinctly higher.
//	     Form uses a 60/40 season/recent blend; attack/creativity use same blend.
//	     Weights: form=0.22, atk=0.08, cre=0.07, def=0.30, avail=0.13, disc=0.10, opp=0.10 → 1.00
//
//	MID: Most balanced role. Creativity includes pass accuracy bonus (tidy MIDs
//	     score higher). Defensive component now uses two signals (duel + tackle rate)
//	     instead of one, giving better spread between DMs and AMs.
//	     Weights: form=0.20, atk=0.17, cre=0.24, def=0.12, avail=0.10, disc=0.09, opp=0.08 → 1.00
//
//	FWD: Attack dominates (0.33) with recent xG/goals blended in.
//	     Opponent weight raised to 0.12 — strikers are the most affected by defensive
//	     quality. Defensive contribution minimal (0.02): hold-up play only.
//	     Weights: form=0.18, atk=0.33, cre=0.17, def=0.02, avail=0.10, disc=0.08, opp=0.12 → 1.00
func (s *PredictionService) calcPrediction(player models.Player) *models.PlayerPrediction {
	form := formComponent(player)
	attack := attackComponent(player)
	creativity := creativityComponent(player)
	defensive := defensiveComponent(player)
	availability := availabilityComponent(player)
	discipline := disciplineComponent(player)
	opponent := player.OpponentScore
	if opponent == 0 {
		opponent = 5.0
	}

	// ── Sub-role inference ────────────────────────────────────────────────────
	// We only have four positions (GK/DEF/MID/FWD) but real footballers play very
	// different roles within each. We infer the sub-role from the actual stat
	// signature and adjust weights accordingly. This prevents a DM being penalised
	// by the creativity-heavy weights designed for an AM, and stops a CB being
	// unfairly compared against attacking full-backs on the same weight set.
	gamesF := math.Max(1, float64(player.GamesPlayed))
	mins90F := math.Max(1, float64(player.MinutesPlayed)/90.0)

	// DEF sub-role: attacking full-back vs center-back.
	// Attacking FB: ≥1.0 key passes per game OR ≥0.15 assists per game.
	// CB: primarily defensive, creativity contribution is minimal.
	defKPperGame := float64(player.KeyPasses) / gamesF
	defAssistsPerGame := float64(player.Assists) / gamesF
	isAttackingFB := defKPperGame >= 1.0 || defAssistsPerGame >= 0.15

	// MID sub-role: holding/defensive mid vs attacking/box-to-box mid.
	// DM: high duel volume per 90 (≥6) AND low key pass output (<1.5/game).
	// AM/box-to-box: creativity is primary, defensive work is secondary.
	midDuelsPer90 := float64(player.DuelsTotal) / mins90F
	midKPperGame := float64(player.KeyPasses) / gamesF
	isDM := midDuelsPer90 >= 6.0 && midKPperGame < 1.5

	// FWD sub-role: second striker / false nine vs pure striker.
	// Creative FWD: creativity score ≥5.5 OR ≥0.20 assists per game.
	// Pure striker: attack-first, creativity is supplementary.
	fwdAssistsPerGame := float64(player.Assists) / gamesF
	isCreativeFWD := creativity >= 5.5 || fwdAssistsPerGame >= 0.20

	var predicted float64
	var numerator float64
	var denom float64
	switch player.Position {
	case "GK":
		// Defensive dominates (0.60). Pass accuracy now in the defensive component
		// (Signal 5) so GK distribution quality is already captured there.
		numerator = form*0.22 +
			defensive*0.60 +
			availability*0.06 +
			discipline*0.04 +
			opponent*0.08
		denom = 0.22 + 0.60 + 0.06 + 0.04 + 0.08

	case "DEF":
		if isAttackingFB {
			// Attacking full-back: creativity (key passes / crosses) is a real value
			// driver alongside defensive work. Attack weight also rises slightly to
			// capture set-piece contributions and occasional goals.
			// form=0.20 atk=0.10 cre=0.14 def=0.28 avail=0.12 disc=0.08 opp=0.08 → 1.00
			numerator = form*0.20 +
				attack*0.10 +
				creativity*0.14 +
				defensive*0.28 +
				availability*0.12 +
				discipline*0.08 +
				opponent*0.08
			denom = 0.20 + 0.10 + 0.14 + 0.28 + 0.12 + 0.08 + 0.08
		} else {
			// Center-back: defensive dominates; creativity and attack near-zero weights
			// reflect that CB value is almost entirely defensive.
			// form=0.22 atk=0.05 cre=0.05 def=0.35 avail=0.13 disc=0.10 opp=0.10 → 1.00
			numerator = form*0.22 +
				attack*0.05 +
				creativity*0.05 +
				defensive*0.35 +
				availability*0.13 +
				discipline*0.10 +
				opponent*0.10
			denom = 0.22 + 0.05 + 0.05 + 0.35 + 0.13 + 0.10 + 0.10
		}

	case "MID":
		if isDM {
			// Defensive midfielder: ball-winning and press coverage is primary.
			// Defensive weight almost doubles vs the AM weights; creativity drops.
			// form=0.20 atk=0.10 cre=0.14 def=0.24 avail=0.11 disc=0.11 opp=0.10 → 1.00
			numerator = form*0.20 +
				attack*0.10 +
				creativity*0.14 +
				defensive*0.24 +
				availability*0.11 +
				discipline*0.11 +
				opponent*0.10
			denom = 0.20 + 0.10 + 0.14 + 0.24 + 0.11 + 0.11 + 0.10
		} else {
			// Attacking / box-to-box mid: creativity is primary.
			// form=0.20 atk=0.17 cre=0.24 def=0.12 avail=0.10 disc=0.09 opp=0.08 → 1.00
			numerator = form*0.20 +
				attack*0.17 +
				creativity*0.24 +
				defensive*0.12 +
				availability*0.10 +
				discipline*0.09 +
				opponent*0.08
			denom = 0.20 + 0.17 + 0.24 + 0.12 + 0.10 + 0.09 + 0.08
		}

	default: // FWD
		if isCreativeFWD {
			// Second striker / false nine: creativity and attack weighted more equally.
			// form=0.18 atk=0.27 cre=0.23 def=0.02 avail=0.10 disc=0.08 opp=0.12 → 1.00
			numerator = form*0.18 +
				attack*0.27 +
				creativity*0.23 +
				defensive*0.02 +
				availability*0.10 +
				discipline*0.08 +
				opponent*0.12
			denom = 0.18 + 0.27 + 0.23 + 0.02 + 0.10 + 0.08 + 0.12
		} else {
			// Pure striker: attack dominates; creativity supplementary.
			// form=0.18 atk=0.35 cre=0.14 def=0.02 avail=0.10 disc=0.08 opp=0.13 → 1.00
			numerator = form*0.18 +
				attack*0.35 +
				creativity*0.14 +
				defensive*0.02 +
				availability*0.10 +
				discipline*0.08 +
				opponent*0.13
			denom = 0.18 + 0.35 + 0.14 + 0.02 + 0.10 + 0.08 + 0.13
		}
	}
	if denom <= 0 {
		predicted = 6.0
	} else {
		predicted = numerator / denom
	}

	// Sample-size confidence dampening — pulls the score toward the position-neutral
	// (5.5) for players with few games. Without this, a player who scored a hat-trick
	// across their first 3 games would rank above established regulars purely because
	// 3-game per-90 stats are extreme.
	//
	// Confidence rises linearly from 0.55 at 3 games to 1.00 at 20 games.
	// In "recent" mode all players have GamesPlayed=3 (equal dampening → no ranking
	// effect in recent mode, but raw scores are slightly stabilised).
	//
	//   3 games  → 55% actual + 45% neutral(5.5)
	//   10 games → 87% actual + 13% neutral
	//   20+ games→ 100% actual (no dampening)
	const neutralScore = 5.5
	confidence := math.Min(1.0, 0.55+float64(player.GamesPlayed-3)*(0.45/17.0))
	predicted = predicted*confidence + neutralScore*(1.0-confidence)

	// Venue advantage: home teams win ~45% of matches vs ~27% for away teams.
	// A modest modifier (+0.25 home / -0.20 away) is scaled by confidence so
	// small-sample players aren't over-boosted by venue alone.
	// No modifier when IsHome is false AND the next opponent is unknown (default zero value).
	if player.NextOpponent != "" {
		if player.IsHome {
			predicted += 0.25 * confidence
		} else {
			predicted -= 0.20 * confidence
		}
	}

	// Inactivity / low-minutes penalties: both drag the score toward a below-neutral
	// baseline (2.5) so players who aren't playing — or are only receiving cameo
	// minutes — never rank positively on stale or limited stats.
	//   • inactivityFactor: long absence (days since last match)
	//   • recentMinutesFactor: playing but only getting short stints
	// The two cases are mutually exclusive in practice, so both are applied
	// independently without accumulating an unfair double-penalty.
	const inactivityBaseline = 2.5
	if f := inactivityFactor(player.LastMatchDate); f < 1.0 {
		predicted = predicted*f + inactivityBaseline*(1.0-f)
	}
	if f := recentMinutesFactor(player); f < 1.0 {
		predicted = predicted*f + inactivityBaseline*(1.0-f)
	}

	predicted = math.Max(0, math.Min(10, predicted))

	// Keep more precision to reduce artificial ties after normalization.
	predicted = math.Round(predicted*1000) / 1000

	risk := riskLevelFromPredictedScore(predicted)

	hiddenGem, gemReasons := isHiddenGem(player, predicted, attack, creativity)

	return &models.PlayerPrediction{
		Player:            player,
		PredictedScore:    predicted,
		RiskLevel:         risk,
		HiddenGem:         hiddenGem,
		HiddenGemReasons:  gemReasons,
		FormContribution:  math.Round(form*100) / 100,
		ThreatContribution: math.Round(attack*100) / 100,
		OpponentDifficulty: math.Round(opponent*100) / 100,
		MinutesLikelihood: math.Round(availability*100) / 100,
		HomeAwayFactor:    math.Round(defensive*100) / 100,
	}
}
