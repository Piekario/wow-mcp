// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package progress

import "testing"

func TestEtaShortAndLong(t *testing.T) {
	if got := eta(126, 11750); got != "under 1 min" {
		t.Errorf("short time: %q", got)
	}
	if got := eta(1000, 0); got != "" {
		t.Errorf("without a rate: %q", got)
	}
	if got := eta(6000, 6000); got != "1 h" {
		t.Errorf("1 h: %q", got)
	}
}
