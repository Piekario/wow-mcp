// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package cooldowns covers long cooldowns: abilities, items (Hearthstone)
// and profession recipes, for one character or the whole account.
package cooldowns

import (
	"sort"
	"time"

	"wowmcp/internal/snapshot"
)

// Query is the input of wow_cooldowns.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	All       bool   `json:"all,omitempty" jsonschema:"true = cooldowns of all characters on the account"`
}

// Source is a cooldown as the dump has it. Profession is set for recipe cooldowns.
type Source struct {
	Name, Kind, Profession string
	ReadyAt                int64
}

// Entry is a cooldown judged against the clock. Left is rounded up to the minute and zero when Ready.
type Entry struct {
	Source
	Ready bool
	Left  time.Duration
}

// Judge combines the cooldowns from the dump (abilities and items) with the recipe ones, in that order. A name counts
// once (the first wins), and the result goes by ready time.
func Judge(sources []Source, fresh snapshot.Freshness) []Entry {
	var out []Entry
	seen := map[string]bool{}
	for _, src := range sources {
		if src.Name == "" || seen[src.Name] {
			continue
		}
		seen[src.Name] = true
		left, ready := fresh.Until(src.ReadyAt)
		e := Entry{Source: src, Ready: ready}
		if !ready {
			e.Left = left + time.Minute - time.Nanosecond
		}
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return snapshot.FormatTime(out[i].ReadyAt) < snapshot.FormatTime(out[j].ReadyAt)
	})
	return out
}

// ReadyEntries are the names of the cooldowns that have already passed.
func ReadyEntries(entries []Entry) []string {
	var ready []string
	for _, e := range entries {
		if e.Ready {
			ready = append(ready, e.Name)
		}
	}
	return ready
}
