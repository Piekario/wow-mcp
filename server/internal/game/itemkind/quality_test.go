// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package itemkind

import "testing"

func TestQualityName(t *testing.T) {
	cases := []struct {
		q    int64
		want string
		ok   bool
	}{
		{0, "Poor", true},
		{4, "Epic", true},
		{5, "Legendary", true},
		{6, "Artifact", true},
		{7, "Heirloom", true},
		{-1, "", false},
		{8, "", false},
	}
	for _, c := range cases {
		name, ok := QualityName(c.q)
		if name != c.want || ok != c.ok {
			t.Errorf("QualityName(%d) = %q, %v; want %q, %v", c.q, name, ok, c.want, c.ok)
		}
	}
}
