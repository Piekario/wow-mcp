// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package mail

import (
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
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

func TestInspectJudgesExpiryAgainstTheClock(t *testing.T) {
	s := fixtures.Forever(t)
	soon := Inspect(s, fixtures.At(2*time.Hour)).ExpiringSoon()
	if len(soon) != 1 || soon[0].Subject != "Auction successful: Copper Ore" || soon[0].Expiry.Expired {
		t.Errorf("expiring soon = %+v, want the one auction letter", soon)
	}
	late := Inspect(s, fixtures.At(3*24*time.Hour))
	if !late.Letters[0].Expiry.Expired || len(late.ExpiringSoon()) != 0 {
		t.Errorf("three days later the letter is gone, not expiring soon: %+v", late.Letters[0].Expiry)
	}
	if Inspect(snapshot.Snapshot{}, fixtures.At(0)).Saved {
		t.Error("a dump without the mail section is not a saved mailbox")
	}
}
