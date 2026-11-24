// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package ledger

import (
	"strings"

	"wowmcp/internal/game/auctions"
)

// addPrices stores the auction prices of a dump. The addon carries a price across saves and every character of the
// realm sees the same auction house, so an observation is identified by realm, item and time and stored once.
func (l *Ledger) addPrices(realm string, observations []auctions.Observation) error {
	if len(observations) == 0 {
		return nil
	}
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, o := range observations {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO prices (realm, item, seen_at, min_buyout, min_bid) VALUES (?, ?, ?, ?, ?)`,
			realm, o.Item, o.SeenAt, o.MinBuyout, o.MinBid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Prices implements auctions.PriceLog.
func (l *Ledger) Prices(realm string, items []string) ([]auctions.Observation, error) {
	if len(items) == 0 {
		return nil, nil
	}
	args := []any{realm}
	for _, item := range items {
		args = append(args, item)
	}
	rows, err := l.db.Query(`SELECT item, seen_at, min_buyout, min_bid FROM prices
		WHERE realm = ? AND item IN (?`+strings.Repeat(", ?", len(items)-1)+`) ORDER BY seen_at, item`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []auctions.Observation
	for rows.Next() {
		var o auctions.Observation
		if err := rows.Scan(&o.Item, &o.SeenAt, &o.MinBuyout, &o.MinBid); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
