package services

import (
	"math"
	"prediplay/backend/models"
)

// isHiddenGem identifies players whose underlying metrics significantly outpace
// their visible returns — suggesting untapped potential or a breakout incoming.
//
// Requirements: predicted score in the 4.5-8.0 band (not too weak, not already elite)
// AND fewer than a position-typical G+A/goal output (not yet well-known or expensively priced in).
//
// Seven independent signals — any single one qualifies the player:
//
//  1. xG+xA/90 outpaces actual G+A/90 by ≥40%: underlying quality is real,
//     luck or finishing efficiency hasn't caught up. Regression to mean favours them.
//
//  2. High shot volume with below-average conversion vs shots on target:
//     consistently getting into shooting positions; finishing will click.
//
//  3. High creativity score but very few assists relative to key passes:
//     team-mates are spurning the chances, not the player. Assists due.
//
//  4. Strong attack component but low returns for the player's position:
//     quality threat not yet visible in the stat line (new team, early season etc).
//
//  5. High xG per shot (≥0.12): the player takes shots from premium positions
//     (inside the box, one-on-ones) but hasn't scored yet. Quality > quantity.
//
//  6. Improving trajectory: recent xG+xA/90 is ≥30% above the season average,
//     meaning underlying form is genuinely trending up right now.
//
//  7. Position-specific specialist quality (see inline comments per position).
func isHiddenGem(p models.Player, predicted, attackScore, creativityScore float64) (bool, []string) {
	if predicted < 4.5 || predicted >= 8.0 {
		return false, nil
	}

	// Position-aware thresholds:
	// - defenders naturally have fewer goals/assists than mids/forwards
	// - forwards can still be "hidden gems" with slightly higher totals
	//   if they are underperforming their underlying xG/xA signals.
	maxGATotal := 12
	lowReturns := 6
	lowGoals := 3
	switch p.Position {
	case "GK":
		maxGATotal = 6
		lowReturns = 1
		lowGoals = 1
	case "DEF":
		maxGATotal = 10
		lowReturns = 4
		lowGoals = 2
	case "MID":
		maxGATotal = 12
		lowReturns = 6
		lowGoals = 3
	default: // FWD
		maxGATotal = 14
		lowReturns = 8
		lowGoals = 5
	}

	if p.Goals+p.Assists >= maxGATotal {
		return false, nil // already well-known / priced in
	}
	mins90 := per90(p.MinutesPlayed)
	xgXaPer90 := (p.XG + p.XA) / mins90
	gAPer90 := float64(p.Goals+p.Assists) / mins90

	// Signal 1: expected stats far exceed actual returns
	underperformingExpected := xgXaPer90 >= 0.25 && xgXaPer90 > gAPer90*1.40

	// Signal 2: high shot volume, conversion hasn't clicked yet
	prolificShooter := p.TotalShots >= 15 && p.ShotsOnTarget > 0 &&
		float64(p.Goals) < float64(p.ShotsOnTarget)*0.22

	// Signal 3: creative output not rewarded with assists
	creativeButUnrewarded := creativityScore >= 5.0 && p.KeyPasses > 0 &&
		float64(p.Assists) < float64(p.KeyPasses)*0.15

	// Signal 4: genuine attack threat, returns not there yet
	highThreatLowReturns := attackScore >= 5.0 && p.Goals+p.Assists < lowReturns

	// Signal 5: taking high-quality shots but not converting yet
	highQualityPositions := false
	if p.TotalShots >= 6 {
		xgPerShot := p.XG / float64(p.TotalShots)
		highQualityPositions = xgPerShot >= 0.12 && p.Goals < lowGoals
	}

	// Signal 6: recent underlying stats trending clearly upward.
	// Threshold lowered from 1.30 to 1.25 — a 25% improvement in recent xG+xA/90
	// is already a meaningful signal of an upswing; 30% was missing gradual momentum shifts.
	improvingTrajectory := false
	recentInScoringView := p.RecentGamesPlayed > 0 && p.RecentMinutes == p.MinutesPlayed
	if !recentInScoringView && p.RecentGamesPlayed >= 3 && p.RecentMinutes > 0 && xgXaPer90 > 0.05 {
		recentXT90 := (p.RecentXG + p.RecentXA) / math.Max(1, float64(p.RecentMinutes)/90.0)
		improvingTrajectory = recentXT90 > xgXaPer90*1.25
	}

	// Signal 7: position-specific specialist quality that standard signals miss.
	//
	//  GK:  Underrated keeper on a struggling team. High saves/game + high goals
	//       conceded/game + good save rate = GK bailing out a poor defence.
	//       Strong upside if the team improves or the keeper moves.
	//
	//  DEF: Ball-playing defender. Elite pass accuracy (≥87%) with solid duel win
	//       rate signals a modern CB/FB whose build-up value isn't visible in G+A.
	//
	//  MID: Box-to-box engine. High pressing volume (≥5 duels/90) combined with
	//       meaningful creativity flags an all-action midfielder whose work rate
	//       and range are undervalued by casual G+A-based assessment.
	//
	//  FWD: Pure poacher. xG per shot ≥0.16 means the player consistently occupies
	//       premium positions (penalty area, one-on-ones) — a higher bar than signal 5.
	positionGem := false
	positionGemReason := ""
	switch p.Position {
	case "GK":
		if p.GamesPlayed >= 3 && p.Saves > 0 {
			savesPerGame := float64(p.Saves) / float64(p.GamesPlayed)
			gcPerGame := float64(p.GoalsConceded) / float64(p.GamesPlayed)
			total := float64(p.Saves + p.GoalsConceded)
			saveRate := float64(p.Saves) / total
			if savesPerGame >= 3.0 && gcPerGame >= 1.2 && saveRate >= 0.68 {
				positionGem = true
				positionGemReason = "Overperforming behind a weak defence"
			}
		}
	case "DEF":
		if p.TotalPasses >= 30 && p.GamesPlayed >= 3 {
			passAcc := float64(p.AccuratePasses) / float64(p.TotalPasses)
			duelRate := safeRate(p.DuelsWon, p.DuelsTotal)
			if passAcc >= 0.87 && duelRate >= 0.52 && p.Goals+p.Assists < 5 {
				positionGem = true
				positionGemReason = "Ball-playing defender with elite distribution"
			}
		}
	case "MID":
		if p.GamesPlayed >= 3 {
			duelsPer90 := float64(p.DuelsTotal) / mins90
			if duelsPer90 >= 5.0 && creativityScore >= 4.5 && p.Goals+p.Assists < 8 {
				positionGem = true
				positionGemReason = "High-energy engine with creative upside"
			}
		}
	default: // FWD
		if p.TotalShots >= 10 {
			xgPerShot := p.XG / float64(p.TotalShots)
			if xgPerShot >= 0.16 && p.Goals < lowGoals {
				positionGem = true
				positionGemReason = "Premium shooting positions, conversion incoming"
			}
		}
	}

	reasons := make([]string, 0, 4)
	if underperformingExpected {
		reasons = append(reasons, "Expected threat is real (xG+xA > G+A)")
	}
	if prolificShooter {
		reasons = append(reasons, "High chances, low conversion")
	}
	if creativeButUnrewarded {
		reasons = append(reasons, "Creating well, assists lag")
	}
	if highThreatLowReturns {
		reasons = append(reasons, "Strong threat, few returns")
	}
	if highQualityPositions {
		reasons = append(reasons, "Quality shots, not finishing yet")
	}
	if improvingTrajectory {
		reasons = append(reasons, "Underlying trend is improving")
	}
	if positionGem {
		reasons = append(reasons, positionGemReason)
	}

	if len(reasons) == 0 {
		return false, nil
	}
	return true, reasons
}

// calcRedFlag always receives the full player (both overall and recent stats intact)
// so it can detect true decline rather than just absolute badness.
//
// Ten signals are computed and combined with position-specific weights.
// The composite produces a 0-10 alarm score; ≥4.0 is shown to users.
//
// FormDecline calibration rationale:
//
//	absFormBad uses 7.0 as the "good form" baseline (not 6.5).
//	Formula: (7.0 - recentForm) / 3.0 * 10 — so:
//	  recentForm 7.0 → 0 (fine), 6.0 → 3.3, 5.0 → 6.7 (alarming), 4.0 → 10.
//	The old formula using 6.5 as baseline gave recentForm=5.0 only 2.3 — far too lenient.
//
//	relFormDecline uses an absolute-point scale: each rating point dropped scores 2.5.
//	A 1-point drop (e.g. 7.5 → 6.5) = 2.5; a 2-point drop = 5.0.
//	Old formula divided by season average, meaning a drop from 7.5 to 6.5 scored 1.3 — too lenient.
func calcRedFlag(p models.Player) (score, formDecline, outputDrop float64, reasons []string) {
	mins90 := per90(p.MinutesPlayed)
	recentMins90 := math.Max(0.5, float64(p.RecentMinutes)/90.0)

	// ── 1. Form decline ───────────────────────────────────────────────────────
	overallForm := p.FormScore
	if overallForm <= 0 {
		overallForm = 6.0
	}
	recentForm := p.RecentFormScore
	if recentForm <= 0 {
		recentForm = 6.0
	}
	// Absolute: distance below 7.0 "good form" baseline — calibrated so 5.0 = alarming (6.7)
	absFormBad := math.Max(0, (7.0-recentForm)/3.0*10)
	// Relative: each rating point dropped scores 2.5 (1pt drop=2.5, 2pt=5.0, 4pt=10)
	relFormDecline := 0.0
	if overallForm > recentForm {
		relFormDecline = math.Min(10, (overallForm-recentForm)*2.5)
	}
	formDecline = math.Min(10, math.Max(absFormBad, relFormDecline))

	// ── 2. Attacking output drop ──────────────────────────────────────────────
	overallGA90 := float64(p.Goals+p.Assists) / mins90
	recentGA90 := float64(p.RecentGoals+p.RecentAssists) / recentMins90

	var posBaseline float64
	switch p.Position {
	case "GK":
		posBaseline = 0
	case "DEF":
		posBaseline = 0.10
	case "MID":
		posBaseline = 0.22
	default:
		posBaseline = 0.35
	}
	absOutputBad := 0.0
	if p.Position != "GK" && posBaseline > 0 {
		absOutputBad = math.Max(0, (posBaseline-recentGA90)/posBaseline*9)
	}
	relOutputDecline := 0.0
	if overallGA90 > 0.06 {
		relOutputDecline = math.Max(0, (overallGA90-recentGA90)/overallGA90*10)
	}
	outputDrop = math.Min(10, math.Max(absOutputBad, relOutputDecline))

	// ── 3. Expected-threat decline (xG+xA per 90) ────────────────────────────
	overallXT90 := (p.XG + p.XA) / mins90
	recentXT90 := (p.RecentXG + p.RecentXA) / recentMins90
	xThreatDecline := 0.0
	if p.Position != "GK" && overallXT90 > 0.06 {
		xThreatDecline = math.Max(0, (overallXT90-recentXT90)/overallXT90*10)
	}

	// ── 4. Shot accuracy decline ──────────────────────────────────────────────
	shotAccDecline := 0.0
	if p.TotalShots >= 10 && p.RecentTotalShots > 0 && p.Position != "GK" {
		overallShotAcc := float64(p.ShotsOnTarget) / float64(p.TotalShots)
		recentShotAcc := float64(p.RecentShotsOnTarget) / float64(p.RecentTotalShots)
		if overallShotAcc > 0.10 {
			shotAccDecline = math.Max(0, (overallShotAcc-recentShotAcc)/overallShotAcc*10)
		}
	}

	// ── 5. Passing / involvement decline ─────────────────────────────────────
	involvementDecline := 0.0
	if p.TotalPasses >= 20 && p.RecentTotalPasses > 0 {
		overallPassAcc := float64(p.AccuratePasses) / float64(p.TotalPasses)
		recentPassAcc := float64(p.RecentAccuratePasses) / float64(p.RecentTotalPasses)
		if overallPassAcc > 0.50 {
			involvementDecline = math.Max(0, (overallPassAcc-recentPassAcc)/overallPassAcc*9)
		}
	}

	// ── 6. Discipline risk (recent period only) ───────────────────────────────
	disciplineRisk := math.Min(10, float64(p.RecentYellowCards)*2.5+float64(p.RecentRedCards)*8.0)

	// ── 7. GK-specific: save rate and goals conceded ──────────────────────────
	gkDecline := 0.0
	if p.Position == "GK" {
		overallGKTotal := float64(p.Saves + p.GoalsConceded)
		recentGKTotal := float64(p.RecentSaves + p.RecentGoalsConceded)
		if overallGKTotal >= 5 && recentGKTotal >= 1 {
			overallSaveRate := float64(p.Saves) / overallGKTotal
			recentSaveRate := float64(p.RecentSaves) / recentGKTotal
			if overallSaveRate > 0.40 {
				gkDecline = math.Max(0, (overallSaveRate-recentSaveRate)/overallSaveRate*10)
			}
		}
		gcPerGame := float64(p.RecentGoalsConceded) / math.Max(1, float64(p.RecentGamesPlayed))
		if gcPerGame >= 2.5 {
			gkDecline = math.Max(gkDecline, math.Min(10, (gcPerGame-1.5)*4))
		}
	}

	// ── 8. DEF: duel win rate decline (losing ground in physical battles) ────
	// Compares recent duel win rate against the season average.
	// A defender who was winning 58% of duels all season but is now at 42% is a
	// concrete defensive red flag — opponents are increasingly getting past them.
	duelWinDecline := 0.0
	if p.Position == "DEF" && p.DuelsTotal >= 15 && p.RecentDuelsTotal >= 4 {
		overallDuelRate := float64(p.DuelsWon) / float64(p.DuelsTotal)
		recentDuelRate := float64(p.RecentDuelsWon) / float64(p.RecentDuelsTotal)
		if overallDuelRate > 0.45 {
			duelWinDecline = math.Max(0, (overallDuelRate-recentDuelRate)/overallDuelRate*10)
		}
	}

	// ── 9. MID: key pass contribution decline (creative influence fading) ─────
	// A midfielder whose key passes per 90 have dropped sharply is losing their
	// creative impact — often an early sign of fatigue, loss of role, or confidence.
	keyPassDecline := 0.0
	if p.Position == "MID" && p.KeyPasses >= 8 && p.RecentMinutes > 0 {
		overallKP90 := float64(p.KeyPasses) / mins90
		recentKP90 := float64(p.RecentKeyPasses) / recentMins90
		if overallKP90 > 0.4 {
			keyPassDecline = math.Max(0, (overallKP90-recentKP90)/overallKP90*10)
		}
	}

	// ── 10. FWD: xG per shot decline (moving to worse shooting positions) ─────
	// A striker whose xG per shot is falling is no longer getting into premium
	// positions — suggesting defensive marking, loss of runs, or tactical demotion.
	xGPerShotDecline := 0.0
	if p.Position == "FWD" && p.TotalShots >= 10 && p.RecentTotalShots >= 3 {
		overallXGperShot := p.XG / float64(p.TotalShots)
		recentXGperShot := p.RecentXG / float64(p.RecentTotalShots)
		if overallXGperShot > 0.08 {
			xGPerShotDecline = math.Max(0, (overallXGperShot-recentXGperShot)/overallXGperShot*10)
		}
	}

	// ── Composite (position-weighted) ────────────────────────────────────────
	switch p.Position {
	case "GK":
		score = formDecline*0.25 + gkDecline*0.45 + disciplineRisk*0.10 + involvementDecline*0.20
	case "DEF":
		// duelWinDecline replaces some of xThreatDecline weight — defenders rarely
		// generate xT themselves, but losing duels is a direct defensive red flag.
		score = formDecline*0.22 + outputDrop*0.15 + xThreatDecline*0.10 +
			involvementDecline*0.18 + disciplineRisk*0.20 + duelWinDecline*0.15
	case "MID":
		// keyPassDecline gets its own slice because midfield creativity is a primary
		// value driver — fading key passes often precede a full output decline.
		score = formDecline*0.22 + outputDrop*0.18 + xThreatDecline*0.14 +
			shotAccDecline*0.08 + involvementDecline*0.14 + disciplineRisk*0.10 + keyPassDecline*0.14
	default: // FWD
		// xGPerShotDecline replaces some outputDrop weight — for forwards, declining
		// shot quality is an earlier and more predictive signal than raw G+A drop.
		score = formDecline*0.21 + outputDrop*0.24 + xThreatDecline*0.17 +
			shotAccDecline*0.12 + involvementDecline*0.07 + disciplineRisk*0.04 + xGPerShotDecline*0.15
	}
	// Keep more precision to reduce artificial ties after normalization.
	score = math.Round(math.Min(10, score)*1000) / 1000

	// ── Reason strings ────────────────────────────────────────────────────────
	// Thresholds are lower than the old version to surface real concerns earlier.
	if formDecline >= 6.5 {
		reasons = append(reasons, "Form has collapsed")
	} else if formDecline >= 3.5 {
		reasons = append(reasons, "Noticeable form decline")
	} else if recentForm < 6.0 {
		reasons = append(reasons, "Below-average recent form")
	}

	if p.Position != "GK" {
		if outputDrop >= 7 {
			reasons = append(reasons, "Output has completely dried up")
		} else if outputDrop >= 4 {
			reasons = append(reasons, "Significant drop in goal/assist returns")
		}
		if xThreatDecline >= 5 {
			reasons = append(reasons, "xG+xA contribution sharply down")
		}
		if shotAccDecline >= 5 {
			reasons = append(reasons, "Shot accuracy falling off")
		}
		if p.RecentGoals+p.RecentAssists == 0 && p.RecentMinutes >= 180 {
			reasons = append(reasons, "No returns across last 3 games")
		}
		// Specific to attacking positions: not even testing the keeper
		if p.RecentShotsOnTarget == 0 && p.RecentMinutes >= 180 &&
			(p.Position == "FWD" || p.Position == "MID") {
			reasons = append(reasons, "Zero shots on target in last 3 games")
		}
	}
	if p.Position == "GK" && gkDecline >= 4 {
		reasons = append(reasons, "Save rate declining / conceding more heavily")
	}
	if p.Position == "DEF" && duelWinDecline >= 5 {
		reasons = append(reasons, "Losing more duels recently — defensive reliability dropping")
	}
	if p.Position == "MID" && keyPassDecline >= 5 {
		reasons = append(reasons, "Creative output fading — fewer key passes in recent games")
	}
	if p.Position == "FWD" && xGPerShotDecline >= 5 {
		reasons = append(reasons, "Shooting from worse positions — shot quality declining")
	}
	if involvementDecline >= 5 {
		reasons = append(reasons, "Fading involvement in build-up play")
	}
	if disciplineRisk >= 4 {
		reasons = append(reasons, "Discipline concerns — risk of suspension")
	}

	return
}

// calcBenchwarmer rewards consistency over brilliance. Five components:
//
//  1. Availability  — average minutes per game vs a full 90.
//
//  2. Form consistency — two sub-signals combined 60/40:
//     a. Band score: how close is the season average to the 6.0-7.5 "reliable" band?
//        A player averaging 6.8 scores near 10; one averaging 8.5 or 4.5 scores low.
//     b. Stability score: how close is the recent form to the season average?
//        |FormScore - RecentFormScore| × 4, inverted. Penalises volatile players.
//        A player at 6.8 overall but 4.5 recently is NOT a benchwarmer — they're declining.
//        This signal directly catches what the band check alone misses.
//
//  3. Output reliability — G+A per 90 proximity to a moderate position baseline.
//     Being too far above OR below the baseline reduces score (benchwarmers are steady, not elite).
//
//  4. Passing reliability — pass accuracy proximity to a position-specific target.
//     Reliable players circulate the ball cleanly without errors or heroics.
//
//  5. Discipline — card rate per game (reliable players stay on the pitch).
func calcBenchwarmer(p models.Player) (score float64, label string) {
	games := math.Max(1, float64(p.GamesPlayed))
	mins90 := per90(p.MinutesPlayed)

	// 1. Availability (0-10)
	avgMins := float64(p.MinutesPlayed) / games
	availScore := math.Min(10, avgMins/90.0*10)

	// 2. Form consistency (0-10) — band score + stability score
	form := p.FormScore
	if form <= 0 {
		form = 6.0
	}
	// 2a. Band: distance from 6.75 target (the centre of the reliable 6.0-7.5 band)
	bandScore := math.Max(0, 10-math.Abs(form-6.75)*3.5)
	// 2b. Stability: recent form matches season average (volatile = unreliable)
	stabilityScore := 10.0 // default full if no recent data
	if p.RecentGamesPlayed >= 3 && p.RecentFormScore > 0 {
		recentForm := p.RecentFormScore
		stabilityScore = math.Max(0, 10-math.Abs(form-recentForm)*4)
	}
	formConsistency := bandScore*0.60 + stabilityScore*0.40

	// 3. Output reliability (0-10) — how close to a moderate, steady baseline
	ga90 := float64(p.Goals+p.Assists) / mins90
	var outputReliability float64
	switch p.Position {
	case "GK":
		total := float64(p.Saves + p.GoalsConceded)
		if total >= 3 {
			saveRate := float64(p.Saves) / total
			outputReliability = math.Min(10, saveRate*10)
		} else {
			outputReliability = 6.0
		}
	case "DEF":
		outputReliability = math.Max(0, 10-math.Abs(ga90-0.10)*30)
	case "MID":
		outputReliability = math.Max(0, 10-math.Abs(ga90-0.20)*22)
	default: // FWD
		outputReliability = math.Max(0, 10-math.Abs(ga90-0.30)*18)
	}

	// 4. Passing reliability (0-10) — accuracy close to position baseline
	passReliability := 6.0
	if p.TotalPasses >= 10 {
		passAcc := float64(p.AccuratePasses) / float64(p.TotalPasses)
		var target float64
		switch p.Position {
		case "GK":
			target = 0.60
		case "DEF":
			target = 0.78
		case "MID":
			target = 0.82
		default:
			target = 0.72
		}
		passReliability = math.Max(0, 10-math.Abs(passAcc-target)*22)
	}

	// 5. Discipline (0-10)
	yellowPerGame := float64(p.YellowCards) / games
	redPerGame := float64(p.RedCards) / games
	discipline := math.Max(0, 10-yellowPerGame*5-redPerGame*15)

	// 6. Position-specific speciality reliability (0-10)
	//
	//  GK:  Goals conceded per game close to 0.9/game is the reliable keeper zone —
	//       not so low that they only face routine saves, not so high that defence is
	//       chaotic. Scores 10 at 0.9, drops off steeply in either direction.
	//
	//  DEF: Duel and tackle win rates near 52-58% signal a dependably combative
	//       defender — aggressive enough to win challenges, controlled enough not to
	//       lunge. Both rates contribute (60% duels / 40% tackles).
	//
	//  MID: Key passes per game near 1.5 = reliable creative presence without being
	//       the team's star. Too few = limited; too many = not benchwarmer territory.
	//
	//  FWD: Shots per game near 2.5 = consistent threat without being prolific.
	//       Tests the keeper every game but isn't hogging the ball or over-shooting.
	var specialityScore float64
	switch p.Position {
	case "GK":
		gcPerGame := float64(p.GoalsConceded) / games
		specialityScore = math.Max(0, 10-math.Abs(gcPerGame-0.9)*7)
	case "DEF":
		duelRate := safeRate(p.DuelsWon, p.DuelsTotal)
		tackleRate := safeRate(p.TacklesWon, p.TacklesTotal)
		specialityScore = math.Max(0, 10-math.Abs(duelRate-0.55)*20)*0.60 +
			math.Max(0, 10-math.Abs(tackleRate-0.55)*20)*0.40
	case "MID":
		kpPerGame := float64(p.KeyPasses) / games
		specialityScore = math.Max(0, 10-math.Abs(kpPerGame-1.5)*4)
	default: // FWD
		shotsPerGame := float64(p.TotalShots) / games
		specialityScore = math.Max(0, 10-math.Abs(shotsPerGame-2.5)*2.5)
	}

	// Weighted composite by position
	switch p.Position {
	case "GK":
		// specialityScore (gc/game) replaces some outputReliability weight — goals
		// conceded per game is a more granular reliability signal than raw save rate.
		score = availScore*0.25 + formConsistency*0.22 + outputReliability*0.25 +
			specialityScore*0.15 + discipline*0.13
	case "DEF":
		// specialityScore (duel/tackle rates) captures physical defensive consistency
		// that G+A proximity can't measure.
		score = availScore*0.22 + formConsistency*0.22 + outputReliability*0.15 +
			passReliability*0.15 + specialityScore*0.13 + discipline*0.13
	case "MID":
		// specialityScore (key passes/game) captures steady creative contribution.
		score = availScore*0.18 + formConsistency*0.22 + outputReliability*0.20 +
			passReliability*0.18 + specialityScore*0.12 + discipline*0.10
	default: // FWD
		// specialityScore (shots/game) captures reliable goal threat beyond just G+A.
		score = availScore*0.18 + formConsistency*0.22 + outputReliability*0.25 +
			passReliability*0.13 + specialityScore*0.12 + discipline*0.10
	}
	// Keep more precision to reduce artificial ties after normalization.
	score = math.Round(score*1000) / 1000

	// Position-specific labels make the benchwarmer list feel more meaningful to
	// users — a "Wall Between the Posts" reads very differently from a "Rock Solid"
	// label applied generically across all positions.
	switch p.Position {
	case "GK":
		switch {
		case score >= 7.5:
			label = "Wall Between the Posts"
		case score >= 5.5:
			label = "Reliable Stopper"
		case score >= 4.0:
			label = "Rotation Keeper"
		default:
			label = ""
		}
	case "DEF":
		switch {
		case score >= 7.5:
			label = "Defensive Pillar"
		case score >= 5.5:
			label = "Solid Defender"
		case score >= 4.0:
			label = "Rotation Defender"
		default:
			label = ""
		}
	case "MID":
		switch {
		case score >= 7.5:
			label = "Engine Room"
		case score >= 5.5:
			label = "Steady Midfielder"
		case score >= 4.0:
			label = "Rotation Mid"
		default:
			label = ""
		}
	default: // FWD
		switch {
		case score >= 7.5:
			label = "Reliable Striker"
		case score >= 5.5:
			label = "Impact Substitute"
		case score >= 4.0:
			label = "Rotation Forward"
		default:
			label = ""
		}
	}
	return
}
