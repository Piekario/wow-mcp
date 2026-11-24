// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package eventlog is the timestamped event log the addon keeps while playing (level-ups, gold changes, quests, zones,
// gear changes). The addon carries entries across saves, so the same entry shows up in many consecutive dumps.
package eventlog

import "strconv"

const (
	KindLevel         = "level"
	KindMoney         = "money"
	KindQuestAccepted = "questAccepted"
	KindQuestTurnedIn = "questTurnedIn"
	KindZone          = "zone"
	KindEquipment     = "equipment"
)

// Entry is one event; only the fields of its Kind are set. An emptied equipment slot has ItemID 0.
type Entry struct {
	At      int64  `json:"at"`
	Kind    string `json:"kind"`
	Level   int64  `json:"level,omitempty"`
	Copper  int64  `json:"copper,omitempty"`
	QuestID int64  `json:"questID,omitempty"`
	Title   string `json:"title,omitempty"`
	XP      int64  `json:"xp,omitempty"`
	Zone    string `json:"zone,omitempty"`
	Slot    string `json:"slot,omitempty"`
	ItemID  int64  `json:"itemID,omitempty"`
}

// Key tells entries of the same kind and time apart (two quests turned in within one second are two events).
// A money entry has no key: the addon rewrites its copper while its time stays, so it is one event.
func (e Entry) Key() string {
	switch e.Kind {
	case KindLevel:
		return strconv.FormatInt(e.Level, 10)
	case KindQuestAccepted, KindQuestTurnedIn:
		return strconv.FormatInt(e.QuestID, 10)
	case KindZone:
		return e.Zone
	case KindEquipment:
		return e.Slot
	}
	return ""
}
