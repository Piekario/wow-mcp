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

// Ending is one of the player's own unsold auctions that is about to end. A Left of zero or less means it may
// have expired since the list was saved.
type Ending struct {
	Name  string
	Count int64
	Left  time.Duration
}

// EndingAuctions returns the player's own unsold auctions that end within the given duration.
// Time left is counted from the moment the auction list was saved (ownedAt), because the client reports it relative to that moment.
// Empty when the data carries no time left.
func EndingAuctions(s snapshot.Snapshot, fresh snapshot.Freshness, within time.Duration) []Ending {
	sec := readSection(s)
	var elapsed time.Duration
	if sec.HasOwnedAt {
		elapsed = fresh.Age(sec.OwnedAt)
	}
	var out []Ending
	for _, a := range sec.Owned {
		if !a.HasTimeLeft || a.Sold {
			continue
		}
		left := time.Duration(a.TimeLeft)*time.Second - elapsed
		if left > within {
			continue
		}
		out = append(out, Ending{Name: fmt.Sprint(a.Name), Count: a.Units, Left: left})
	}
	return out
}
