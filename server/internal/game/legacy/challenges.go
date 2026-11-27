// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package legacy

import (
	"wowmcp/internal/game/achievements"
	"wowmcp/internal/snapshot"
)

// Where the challenge list came from.
const (
	ChallengesFromAddon   = "addon"
	ChallengesFromRewards = "achievementRewards"
)

// rewardFragment is how the real client words the reward of a Legacy challenge ("Earn 1 Legacy Point."), English client text.
const rewardFragment = "Legacy Point"

// deriveChallenges finds the Legacy challenges among the dump's achievements by their reward text, for dumps whose
// addon found none itself (addons before 0.21.0 matched only a category named Legacy, which the real client does not
// have). It returns nil when the dump has no achievements or none of them rewards Legacy Points.
func deriveChallenges(s snapshot.Snapshot) *ChallengeReport {
	entries, found := achievements.WithReward(s, rewardFragment)
	if !found || len(entries) == 0 {
		return nil
	}
	list := make([]Challenge, len(entries))
	for i, e := range entries {
		list[i] = Challenge{Name: e.Name, Category: e.Category, Description: e.Description, Reward: e.Reward,
			Points: e.Points, Completed: e.Completed, CompletedOn: e.CompletedOn, Missing: e.Missing, MoreMissing: e.MoreMissing}
	}
	r := summariseChallenges(list)
	r.Source = ChallengesFromRewards
	r.MatchedBy = `derived from the achievements section: reward text contains "Legacy Point" (English client text)`
	return r
}
