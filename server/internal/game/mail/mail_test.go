// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package mail

import (
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
)

func TestMailView(t *testing.T) {
	s := fixtures.Forever(t)
	m := View(s, fixtures.At(2*time.Hour))
	fixtures.WantAll(t, "mail", fixtures.JSON(t, m), `"totalMoney":"54s 0c"`, `"items":["Linen Cloth x20"]`,
		`"expiringSoon":["Auction successful: Copper Ore (2 d 10 h)"]`, `"unread":true`)
	if m := View(s, fixtures.At(3*24*time.Hour)); !strings.Contains(fixtures.JSON(t, m), `"expired":true`) {
		t.Errorf("letter past its expiry: %s", fixtures.JSON(t, m))
	}
}
