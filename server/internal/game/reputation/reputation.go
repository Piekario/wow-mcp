// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package reputation covers faction reputation: standing and progress to the next one.
package reputation

import (
	"math"
	"strings"
)

// Query holds the wow_reputation filters.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Query     string `json:"query,omitempty" jsonschema:"faction name fragment, e.g. Darnassus"`
	Standing  string `json:"standing,omitempty" jsonschema:"only factions with this standing, e.g. Friendly, Honored, Neutral"`
}

// Faction keeps Standing as the client gave it, so the answer shows exactly what the dump holds.
type Faction struct {
	Name                           string
	Standing                       any
	StandingText                   string
	Value, Floor, Ceiling          int64
	HasValue, HasFloor, HasCeiling bool
}

// Progress is the way through the current standing, between its floor and ceiling thresholds.
type Progress struct {
	Into, Span, ToNext int64
	Percent            float64
}

type Status struct {
	Faction
	Progress *Progress
}

// Report is the faction list after the filters. Saved is false when the dump has no reputation section.
type Report struct {
	Saved    bool
	Factions []Status
}

func Inspect(factions []Faction, saved bool, q Query) Report {
	r := Report{Saved: saved}
	for _, f := range factions {
		if q.Query != "" && !strings.Contains(strings.ToLower(f.Name), strings.ToLower(q.Query)) {
			continue
		}
		if q.Standing != "" && !strings.EqualFold(f.StandingText, q.Standing) {
			continue
		}
		r.Factions = append(r.Factions, Status{Faction: f, Progress: f.progress()})
	}
	return r
}

func (f Faction) progress() *Progress {
	if !(f.HasValue && f.HasFloor && f.HasCeiling && f.Ceiling > f.Floor) {
		return nil
	}
	span := f.Ceiling - f.Floor
	return &Progress{
		Into: f.Value - f.Floor, Span: span, ToNext: f.Ceiling - f.Value,
		Percent: math.Round(float64(f.Value-f.Floor)*1000/float64(span)) / 10,
	}
}
