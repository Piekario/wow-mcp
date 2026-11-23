// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package export builds a character summary to paste into Discord (Markdown),
// a forum (BBCode) or as plain text.
package export

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"wowmcp/internal/game/professions"
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

// Item is an equipped item. Name stays as the dump has it.
type Item struct {
	Name         any
	Quality      int64
	HasQuality   bool
	ItemLevel    int64
	HasItemLevel bool
}

// Pet keeps the fields as the dump has them.
type Pet struct {
	Name, Family, Level any
}

// Character is what the summary can say about the character.
type Character struct {
	Name             string
	Level            int64
	Race, Class      any
	Faction          string
	TalentSplit      string
	MainTree         string
	AverageItemLevel float64
	HasItemLevel     bool
	Professions      []professions.Profession
	Pet              *Pet
	Played           string
	Money            int64
	Gear             map[string]Item
	LearnedTalents   []string
}

// Result is the summary text and whether one Discord message can hold it.
type Result struct {
	Format  string
	Text    string
	Length  int
	TooLong bool
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

// Summarise builds the character summary: header, faction, talents, item level,
// professions, pet, play time, optionally gold, and without short the gear and talents.
func Summarise(s snapshot.Snapshot, q Query) (Result, error) {
	f, ok := formatNamed(q.Format)
	if !ok {
		return Result{}, fmt.Errorf("unknown format %q: use discord, forum or text", q.Format)
	}
	c := readCharacter(s)
	w := &sheet{format: f}
	w.title(fmt.Sprintf("%s, %d %v %v", c.Name, c.Level, c.Race, c.Class))
	w.field(labels["faction"], c.Faction)
	w.field(labels["talents"], c.talentLine())
	if c.HasItemLevel {
		w.field(labels["ilvl"], fmt.Sprint(c.AverageItemLevel))
	}
	w.field(labels["professions"], c.professionLine())
	if c.Pet != nil {
		w.field(labels["pet"], fmt.Sprintf("%v (%v), %s %v", c.Pet.Name, c.Pet.Family, labels["level"], c.Pet.Level))
	}
	w.field(labels["played"], c.Played)
	if q.Gold {
		w.field(labels["gold"], snapshot.Money(c.Money).String())
	}
	if !q.Short {
		w.list(labels["gear"], c.gearLines(f))
		w.list(labels["talents"], c.LearnedTalents)
	}
	text := f.finish(strings.TrimRight(w.b.String(), "\n"))
	length := utf8.RuneCountInString(text)
	return Result{Format: f.name(), Text: text, Length: length, TooLong: f.name() == "discord" && length > discordLimit}, nil
}

func (c Character) talentLine() string {
	if c.TalentSplit != "" && c.MainTree != "" {
		return c.TalentSplit + " (" + c.MainTree + ")"
	}
	return c.TalentSplit
}

func (c Character) professionLine() string {
	known := make([]string, len(c.Professions))
	for i, p := range c.Professions {
		known[i] = p.Label()
	}
	return strings.Join(known, ", ")
}

// gearLines lists the equipped gear in character window order: "Head: Name (ilvl 18)".
func (c Character) gearLines(f format) []string {
	var gear []string
	for _, slot := range gearSlots {
		item, ok := c.Gear[slot.key]
		if !ok {
			continue
		}
		line := fmt.Sprintf("%s: %s", slot.label, f.itemName(item))
		if item.HasItemLevel && item.ItemLevel > 0 {
			line += fmt.Sprintf(" (ilvl %d)", item.ItemLevel)
		}
		gear = append(gear, line)
	}
	return gear
}
