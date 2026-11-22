// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package auctions

import (
	"fmt"
	"time"

	"wowmcp/internal/snapshot"
)

// EndingSoonWithin: own auctions with less time left than this are considered ending.
const EndingSoonWithin = 2 * time.Hour

// EndingSoon returns descriptions of the player's own unsold auctions that end within the given duration.
// Time left is counted from the moment the auction list was saved (ownedAt), because the client reports it relative to that moment.
// Empty list when the data carries no time left.
func EndingSoon(s snapshot.Snapshot, fresh snapshot.Freshness, within time.Duration) []string {
	auctions := s.Map("auctions")
	var elapsed time.Duration
	if at, ok := snapshot.AsInt(auctions["ownedAt"]); ok {
		elapsed = fresh.Age(at)
	}
	var out []string
	for _, raw := range snapshot.AsList(auctions["owned"]) {
		a := snapshot.AsMap(raw)
		secs, ok := snapshot.AsInt(a["timeLeftSeconds"])
		if !ok || a["sold"] == true {
			continue
		}
		left := time.Duration(secs)*time.Second - elapsed
		if left > within {
			continue
		}
		out = append(out, fmt.Sprintf("%s x%d (%s)", auctionName(a), snapshot.ItemCount(a), leftLabel(left)))
	}
	return out
}

func auctionName(a map[string]any) string { return fmt.Sprint(a["name"]) }

// leftLabel describes the time left on an auction; negative means the auction may have already expired since the list was saved.
func leftLabel(left time.Duration) string {
	if left <= 0 {
		return "may have already expired"
	}
	return snapshot.FormatAge(left) + " left"
}
