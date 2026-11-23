// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package buffs

import (
	"fmt"
	"strings"

	"wowmcp/internal/snapshot"
)

func read(s snapshot.Snapshot) Dump {
	d := Dump{HasAuras: s["auras"] != nil, EnglishClient: s.EnglishClient(), Class: classOf(s), Pet: petOf(s)}
	d.DumpedAt, d.HasDumpedAt = s.DumpedAt()
	for _, raw := range s.List("auras") {
		if a, ok := readAura(snapshot.AsMap(raw)); ok {
			d.Auras = append(d.Auras, a)
		}
	}
	for _, raw := range s.List("spells") {
		if sp, ok := readSpell(snapshot.AsMap(raw)); ok {
			d.Spells = append(d.Spells, sp)
		}
	}
	return d
}

func readAura(m map[string]any) (Aura, bool) {
	name, _ := m["name"].(string)
	if name == "" {
		return Aura{}, false
	}
	a := Aura{Name: name, Harmful: m["harmful"] == true}
	a.ID, a.HasID = snapshot.AsInt(m["id"])
	a.Icon, _ = snapshot.AsInt(m["icon"])
	a.Stacks, a.HasStacks = snapshot.AsInt(m["stacks"])
	a.ExpiresAt, a.HasExpiresAt = snapshot.AsInt(m["expiresAt"])
	a.Remaining, a.HasRemaining = snapshot.AsInt(m["remaining"])
	return a, true
}

func readSpell(m map[string]any) (Spell, bool) {
	name, _ := m["name"].(string)
	if name == "" {
		return Spell{}, false
	}
	sp := Spell{Name: name, SelfBuff: m["selfBuff"] == true && m["passive"] != true}
	sp.ID, _ = snapshot.AsInt(m["id"])
	sp.Icon, _ = snapshot.AsInt(m["icon"])
	return sp, true
}

// classOf is the class token (HUNTER...); the class name stands in for it only on English clients.
func classOf(s snapshot.Snapshot) string {
	class, _ := s.Character()["classToken"].(string)
	if class == "" && s.EnglishClient() { // the class name is a token only in English (Hunter -> HUNTER)
		class = strings.ToUpper(fmt.Sprint(s.Character()["class"]))
	}
	return class
}

func petOf(s snapshot.Snapshot) PetState {
	if s["pet"] == nil {
		return PetUnknown
	}
	pet := s.Map("pet")
	switch {
	case pet["exists"] == false:
		return PetAbsent
	case pet["exists"] == true && pet["dead"] == true:
		return PetDead
	case pet["exists"] == true:
		return PetAlive
	}
	return PetUnknown
}
