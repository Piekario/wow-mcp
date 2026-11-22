// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package fixtures

import (
	"testing"
	"time"
)

// The testdata path must work from any package, and the clock must be fixed.
func TestFixtures(t *testing.T) {
	if Forever(t).Character()["name"] != "Piekasz" || Modern(t).Character()["name"] != "Piekasz" {
		t.Error("bad test data")
	}
	if got := At(time.Hour).Now(); got.Unix() != ForeverDumpedAt+3600 {
		t.Errorf("clock = %v", got)
	}
}

func TestRealSnapshotLoads(t *testing.T) {
	s := Real(t)
	if got, _ := s.DumpedAt(); got != RealDumpedAt {
		t.Errorf("dumpedAt = %d, want %d", got, RealDumpedAt)
	}
	if level, _ := s.LevelAndMoney(); level != 22 {
		t.Errorf("level = %d, want 22", level)
	}
	if len(s.List("bags")) == 0 || len(s.Map("talents")["nodes"].([]any)) != 52 {
		t.Error("the real snapshot lost its bags or talent nodes")
	}
	if got, _ := Real0110(t).DumpedAt(); got != Real0110DumpedAt {
		t.Errorf("dumpedAt of the addon 0.11.0 dump = %d, want %d", got, Real0110DumpedAt)
	}
	if got, _ := RealUnloaded(t).DumpedAt(); got != RealUnloadedDumpedAt {
		t.Errorf("dumpedAt of the exit dump = %d, want %d", got, RealUnloadedDumpedAt)
	}
}
