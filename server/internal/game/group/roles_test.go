// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package group

import "testing"

func TestClassRoles(t *testing.T) {
	for class, want := range map[string]Role{"PALADIN": Tank, "Paladin": Healer, "death knight": Tank, "DEMON_HUNTER": Tank} {
		if !CanFill(class, want) {
			t.Errorf("%s should be able to fill %s", class, want)
		}
	}
	if CanFill("MAGE", Tank) || CanFill("Hunter", Healer) {
		t.Error("a mage does not tank and a hunter does not heal")
	}
	if ClassRoles("Paladyn") != nil || ClassRoles("") != nil {
		t.Error("an unknown class has no roles")
	}
}
