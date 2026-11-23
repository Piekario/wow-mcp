// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package goals

import (
	"fmt"

	"wowmcp/internal/snapshot"
)

type CharacterState struct{ Level, XP, XPMax int64 }

// Faction is one reputation entry. Standing is passed through as the client gave it.
type Faction struct {
	Name         string
	Standing     any
	StandingName string
	Value        int64
	Number       float64
	HasNumber    bool
	Floor        int64
	HasFloor     bool
}

// Skill is one entry of the professions section (professions and other skills). Display is the name as written
// in messages.
type Skill struct {
	Name, Display string
	Rank, MaxRank int64
	Number        float64
	HasNumber     bool
}

type SpellEntry struct{ Name, Rank string }

type TrainerService struct {
	Name, Rank    string
	Category      any
	Used          bool
	CostCopper    int64
	LevelRequired int64
}

type Trainer struct {
	NPC      any
	Services []TrainerService
}

func readCharacterState(s snapshot.Snapshot) CharacterState {
	c := s.Character()
	var st CharacterState
	st.Level, _ = snapshot.AsInt(c["level"])
	st.XP, _ = snapshot.AsInt(c["xp"])
	st.XPMax, _ = snapshot.AsInt(c["xpMax"])
	return st
}

func readFactions(s snapshot.Snapshot) []Faction {
	var out []Faction
	for _, raw := range s.List("reputation") {
		m := snapshot.AsMap(raw)
		f := Faction{Standing: m["standing"]}
		f.Name, _ = m["name"].(string)
		f.StandingName, _ = m["standing"].(string)
		f.Value, _ = snapshot.AsInt(m["value"])
		f.Number, f.HasNumber = snapshot.AsFloat(m["value"])
		f.Floor, f.HasFloor = snapshot.AsInt(m["floor"])
		out = append(out, f)
	}
	return out
}

func readSkills(s snapshot.Snapshot) []Skill {
	var out []Skill
	for _, raw := range s.List("professions") {
		m := snapshot.AsMap(raw)
		k := Skill{Display: fmt.Sprint(m["name"])}
		k.Name, _ = m["name"].(string)
		k.Rank, _ = snapshot.AsInt(m["rank"])
		k.MaxRank, _ = snapshot.AsInt(m["maxRank"])
		k.Number, k.HasNumber = snapshot.AsFloat(m["rank"])
		out = append(out, k)
	}
	return out
}

func readSpells(s snapshot.Snapshot) []SpellEntry {
	var out []SpellEntry
	for _, raw := range s.List("spells") {
		m := snapshot.AsMap(raw)
		var e SpellEntry
		e.Name, _ = m["name"].(string)
		e.Rank, _ = m["rank"].(string)
		out = append(out, e)
	}
	return out
}

func readTrainer(s snapshot.Snapshot) Trainer {
	section := s.Map("trainer")
	t := Trainer{NPC: section["npc"]}
	for _, raw := range snapshot.AsList(section["services"]) {
		m := snapshot.AsMap(raw)
		svc := TrainerService{Category: m["category"], Used: m["category"] == "used"}
		svc.Name, _ = m["name"].(string)
		svc.Rank, _ = m["rank"].(string)
		svc.CostCopper, _ = snapshot.AsInt(m["costCopper"])
		svc.LevelRequired, _ = snapshot.AsInt(m["levelRequired"])
		t.Services = append(t.Services, svc)
	}
	return t
}
