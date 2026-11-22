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
