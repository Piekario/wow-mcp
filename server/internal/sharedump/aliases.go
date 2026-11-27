// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package sharedump

import (
	"regexp"
	"strconv"
	"strings"
)

// selfAlias replaces the name of the character the dump belongs to.
const selfAlias = "Player"

// aliasForm recognizes a value that is already an alias, so a shared dump can be shared again unchanged.
var aliasForm = regexp.MustCompile(`^(Player|Guild|Sender|Pet|Macro|Set)( \d+)?$`)

func isAlias(name string) bool { return aliasForm.MatchString(name) }

// aliases hands out stable replacements: the same name always gets the same alias within one dump.
type aliases struct {
	players, guilds map[string]string
	local           map[string]map[string]string // alias prefix -> original -> alias
}

func newAliases() *aliases {
	return &aliases{players: map[string]string{}, guilds: map[string]string{}, local: map[string]map[string]string{}}
}

// baseName cuts the realm from "Name-Realm", the form some clients use for other players.
func baseName(name string) string {
	if cut := strings.IndexByte(name, '-'); cut > 0 {
		return name[:cut]
	}
	return name
}

func foldName(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

func (a *aliases) self(name string) {
	if key := foldName(baseName(name)); key != "" && !isAlias(name) {
		a.players[key] = selfAlias
	}
}

func (a *aliases) player(name string) string {
	key := foldName(baseName(name))
	if key == "" {
		return name
	}
	if _, known := a.players[key]; !known {
		a.players[key] = selfAlias + " " + strconv.Itoa(len(a.players)+1)
	}
	return a.players[key]
}

func (a *aliases) guild(name string) string {
	key := foldName(name)
	if key == "" {
		return name
	}
	if _, known := a.guilds[key]; !known {
		a.guilds[key] = "Guild " + strconv.Itoa(len(a.guilds)+1)
	}
	return a.guilds[key]
}

// named gives an alias that holds only in fields of the same kind (prefix), e.g. "Macro 2".
func (a *aliases) named(prefix, name string) string {
	if a.local[prefix] == nil {
		a.local[prefix] = map[string]string{}
	}
	known := a.local[prefix]
	key := foldName(name)
	if _, ok := known[key]; !ok {
		known[key] = prefix + " " + strconv.Itoa(len(known)+1)
	}
	return known[key]
}

// correspondent is the alias of a mail sender or auction owner: the player alias when the name is a known
// player, otherwise a "Sender" alias of its own.
func (a *aliases) correspondent(name string) string {
	if alias, known := a.players[foldName(baseName(name))]; known {
		return alias
	}
	return a.named("Sender", name)
}

// secrets are the texts that must not survive anywhere: player and guild names with their aliases.
func (a *aliases) secrets() map[string]string {
	out := make(map[string]string, len(a.players)+len(a.guilds))
	for name, alias := range a.players {
		out[name] = alias
	}
	for name, alias := range a.guilds {
		out[name] = alias
	}
	return out
}
