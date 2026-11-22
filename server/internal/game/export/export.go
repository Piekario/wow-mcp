// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package export builds a character summary to paste into Discord (Markdown),
// a forum (BBCode) or as plain text.
package export

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"wowmcp/internal/game/character"
	"wowmcp/internal/game/professions"
	"wowmcp/internal/game/talents"
	"wowmcp/internal/snapshot"
)

// Query is the input of wow_export.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Format    string `json:"format,omitempty" jsonschema:"discord (default, Markdown), forum (BBCode) or text"`
	Short     bool   `json:"short,omitempty" jsonschema:"true = without the gear and talent lists, only the header, professions and talent split"`
	Gold      bool   `json:"gold,omitempty" jsonschema:"true = include gold (omitted by default)"`
	English   bool   `json:"english,omitempty" jsonschema:"deprecated and ignored: labels are always in English"`
}

// discordLimit is the maximum number of characters in one Discord message.
const discordLimit = 2000

var labels = map[string]string{
	"faction": "Faction", "talents": "Talents", "ilvl": "Average item level",
	"professions": "Professions", "pet": "Pet", "played": "Played",
	"gold": "Gold", "gear": "Gear", "level": "level",
}

var gearSlots = []struct{ key, label string }{
	{"head", "Head"}, {"neck", "Neck"}, {"shoulder", "Shoulder"}, {"back", "Back"}, {"chest", "Chest"},
	{"wrist", "Wrist"}, {"hands", "Hands"}, {"waist", "Waist"}, {"legs", "Legs"}, {"feet", "Feet"},
	{"finger1", "Ring"}, {"finger2", "Ring"}, {"trinket1", "Trinket"}, {"trinket2", "Trinket"},
	{"mainHand", "Main Hand"}, {"offHand", "Off Hand"}, {"ranged", "Ranged"},
}

// sheet assembles the summary text in the chosen format.
type sheet struct {
	format format
	b      strings.Builder
}

func (w *sheet) title(text string) { w.format.title(&w.b, text) }

func (w *sheet) field(label, value string) {
	if value != "" {
		w.format.field(&w.b, label, value)
	}
}

func (w *sheet) list(label string, items []string) {
	if len(items) > 0 {
		w.format.list(&w.b, label, items)
	}
}

// View builds the character summary: header, faction, talents, item level,
// professions, pet, play time, optionally gold, and without short the gear and talents.
func View(s snapshot.Snapshot, q Query) (map[string]any, error) {
	f, ok := formatNamed(q.Format)
	if !ok {
		return nil, fmt.Errorf("unknown format %q: use discord, forum or text", q.Format)
	}
	c := s.Character()
	level, money := s.LevelAndMoney()
	w := &sheet{format: f}
	label := func(key string) string { return labels[key] }

	name, _ := c["name"].(string)
	w.title(fmt.Sprintf("%s, %d %v %v", name, level, c["race"], c["class"]))
	if faction, ok := c["faction"].(string); ok {
		w.field(label("faction"), faction)
	}
	tal := s.Map("talents")
	if split, main := talents.Split(tal); split != "" {
		value := split
		if main != "" {
			value += " (" + main + ")"
		}
		w.field(label("talents"), value)
	}
	if ilvl, ok := character.AverageItemLevel(s); ok {
		w.field(label("ilvl"), fmt.Sprint(ilvl))
	}
	w.field(label("professions"), strings.Join(professions.Summary(s), ", "))
	if pet := s.Map("pet"); pet != nil && pet["exists"] == true {
		w.field(label("pet"), fmt.Sprintf("%v (%v), %s %v", pet["name"], pet["family"], label("level"), pet["level"]))
	}
	if played := character.Played(s); played != nil {
		if t, ok := played["total"].(string); ok {
			w.field(label("played"), t)
		}
	}
	if q.Gold {
		w.field(label("gold"), snapshot.Money(money).String())
	}
	if !q.Short {
		w.list(label("gear"), gearLines(s, f))
		w.list(label("talents"), talents.Learned(tal))
	}

	text := f.finish(strings.TrimRight(w.b.String(), "\n"))
	out := map[string]any{"format": f.name(), "text": text, "length": utf8.RuneCountInString(text)}
	if f.name() == "discord" && utf8.RuneCountInString(text) > discordLimit {
		out["lengthNote"] = "The text is over 2000 characters, more than one Discord message can hold. Use short=true or paste it in two parts."
	}
	out["howToUse"] = "Show the player the text field in a code block so they can copy it whole."
	return out, nil
}

// gearLines lists the equipped gear in character window order: "Head: Name (ilvl 18)".
func gearLines(s snapshot.Snapshot, f format) []string {
	var gear []string
	bySlot := snapshot.EquippedBySlot(s.Equipment())
	for _, slot := range gearSlots {
		item := bySlot[slot.key]
		if item == nil {
			continue
		}
		line := fmt.Sprintf("%s: %s", slot.label, f.itemName(item))
		if lvl, ok := snapshot.AsInt(item["itemLevel"]); ok && lvl > 0 {
			line += fmt.Sprintf(" (ilvl %d)", lvl)
		}
		gear = append(gear, line)
	}
	return gear
}
