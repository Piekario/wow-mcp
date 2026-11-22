// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package snapshot

import (
	"strings"
	"testing"
)

func TestLookupSameNameOnDifferentRealms(t *testing.T) {
	db := &Database{Last: "Piekasz-Alpha", Characters: map[string]Snapshot{
		"Piekasz-Alpha": {"id": 1}, "Piekasz-Beta": {"id": 2}, "Other-Alpha": {"id": 3},
	}}
	if name, s, err := db.Lookup("Piekasz-Beta"); err != nil || name != "Piekasz-Beta" || s["id"] != 2 {
		t.Errorf("full key: %v %v %v", name, s, err)
	}
	if name, _, err := db.Lookup("other"); err != nil || name != "Other-Alpha" {
		t.Errorf("unique name part, any case: %v %v", name, err)
	}
	_, _, err := db.Lookup("piekasz")
	if err == nil || !strings.Contains(err.Error(), "Piekasz-Alpha, Piekasz-Beta") || !strings.Contains(err.Error(), "Name-Realm") {
		t.Errorf("ambiguous name must list the full keys: %v", err)
	}
	if _, _, err := db.Lookup("Nobody"); err == nil || !strings.Contains(err.Error(), "Available") {
		t.Errorf("unknown: %v", err)
	}
	if name, _, err := db.Lookup(""); err != nil || name != "Piekasz-Alpha" {
		t.Errorf("empty = last dumped: %v %v", name, err)
	}
}
