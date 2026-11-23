// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package cooldowns

import (
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestCooldowns(t *testing.T) {
	s := fixtures.Forever(t)
	cds := For("", s, fixtures.At(10*time.Minute))
	text := fixtures.JSON(t, cds)
	// Aspect of the Hawk (290 s) has already passed, Hearthstone (3000 s) has 40 min left, the recipe 12 h.
	fixtures.WantAll(t, "cooldowns", text, `"name":"Aspect of the Hawk"`, `"ready":true`, `"remaining":"40 min"`,
		`"name":"Cured Light Hide"`, `"profession":"Leatherworking"`, `"remaining":"11 h 50 min"`)
	if snapshot.AsMap(cds[0])["name"] != "Aspect of the Hawk" {
		t.Errorf("order by ready time: %v", cds)
	}
}

func TestJudge(t *testing.T) {
	fresh := fixtures.At(0)
	now := fresh.Now().Unix()
	got := Judge([]Source{
		{Name: "Later", ReadyAt: now + 600},
		{Name: "Done", ReadyAt: now - 60},
		{Name: "Later", ReadyAt: now + 10},
		{Name: "", ReadyAt: now},
	}, fresh)
	if len(got) != 2 || got[0].Name != "Done" || !got[0].Ready || got[1].Ready || got[1].Left != 11*time.Minute-time.Nanosecond {
		t.Errorf("judged: %+v", got)
	}
}
