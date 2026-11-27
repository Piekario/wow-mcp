// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package group

import "strings"

// classRoles is the one table of what each class can do in a group (role capabilities, not a talent choice):
// keyed by class token without spaces (PALADIN, DEATHKNIGHT). The first role is the class's usual one.
// It covers the classic classes and the later ones, so the same table serves every client.
var classRoles = map[string][]Role{
	"WARRIOR":     {Tank, Damage},
	"PALADIN":     {Healer, Tank, Damage},
	"HUNTER":      {Damage},
	"ROGUE":       {Damage},
	"PRIEST":      {Healer, Damage},
	"SHAMAN":      {Healer, Damage},
	"MAGE":        {Damage},
	"WARLOCK":     {Damage},
	"DRUID":       {Healer, Tank, Damage},
	"DEATHKNIGHT": {Tank, Damage},
	"MONK":        {Tank, Healer, Damage},
	"DEMONHUNTER": {Tank, Damage},
	"EVOKER":      {Healer, Damage},
}

// ClassRoles returns the roles a class can take, from its token (PALADIN) or its English name (Death Knight).
// An unknown or localized name gives nil: the caller says the role is unknown instead of guessing.
func ClassRoles(class string) []Role {
	key := strings.ToUpper(strings.NewReplacer(" ", "", "_", "").Replace(class))
	return classRoles[key]
}

// CanFill says whether the class can take the role.
func CanFill(class string, role Role) bool {
	for _, r := range ClassRoles(class) {
		if r == role {
			return true
		}
	}
	return false
}
