// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"testing"

	"wowmcp/internal/snapshot"
)

func TestBagBankStock(t *testing.T) {
	s := snapshot.Snapshot{
		"bags": []any{map[string]any{"items": []any{
			map[string]any{"name": "Light Leather", "type": "Trade Goods", "subType": "Leather", "count": int64(3)},
			map[string]any{"name": "Soul Thing", "count": int64(1)},
		}}},
		"bank": map[string]any{"containers": []any{map[string]any{"items": []any{
			map[string]any{"name": "light leather", "count": int64(4)},
		}}}},
	}
	got := BagBankStock(s, func(it map[string]any) bool { return it["name"] != "Soul Thing" })
	l := got["light leather"]
	if l == nil || l.Bags != 3 || l.Bank != 4 || l.Total() != 7 || l.Name != "Light Leather" || l.SubType != "Leather" {
		t.Fatalf("wrongly summed: %+v", l)
	}
	if _, ok := got["soul thing"]; ok {
		t.Errorf("keep should have skipped the item")
	}
	if all := BagBankStock(s, nil); len(all) != 2 {
		t.Errorf("expected 2 entries without a filter, got %d", len(all))
	}
}
