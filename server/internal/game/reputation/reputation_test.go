// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package reputation

import (
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestReputationView(t *testing.T) {
	out := View(fixtures.Forever(t), Query{Query: "darn"})
	f := snapshot.AsMap(out["factions"].([]any)[0])
	if f["progress"] != "1200/6000" || f["percent"] != 20.0 || f["toNextStanding"] != int64(4800) {
		t.Errorf("reputation progress: %v", f)
	}
	if out := View(fixtures.Forever(t), Query{Standing: "Exalted"}); out["count"] != 0 {
		t.Errorf("standing filter does not work: %v", out)
	}
}
