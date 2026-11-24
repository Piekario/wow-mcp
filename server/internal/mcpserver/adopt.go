// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package mcpserver

import (
	"log"
	"strings"

	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// A character's key changes when the client stops or starts telling the realm: "Name-Realm Name", then "Name", then
// "Name-<realm number>". Its history must follow, or the long history starts again from nothing.

type keyedHistory interface {
	Keys() ([]string, error)
	Adopt(from, to string) error
}

type adoptingLedger interface {
	Adopt(from, to string) error
	Import(character string, files progress.History) error
}

// adoptFormerKeys gives every character that has a realm in its key and no history yet the history kept under its
// former keys. Called before the dump is recorded, so the copied dumps are imported before the keep limit trims them.
func adoptFormerKeys(db *snapshot.Database, history History, ledger Ledger) {
	files, ok := history.(keyedHistory)
	if !ok {
		return
	}
	for _, key := range db.CharacterNames() {
		if snapshot.RealmOf(key) == "" {
			continue
		}
		if times, err := history.List(key); err != nil || len(times) > 0 {
			continue
		}
		former := formerKeys(db, key, history, files)
		for _, old := range former {
			if err := files.Adopt(old, key); err != nil {
				log.Printf("history: adopting %q as %q: %v", old, key, err)
			}
		}
		adoptInLedger(ledger, history, former, key)
	}
}

func adoptInLedger(ledger Ledger, history History, former []string, key string) {
	long, ok := ledger.(adoptingLedger)
	if !ok || len(former) == 0 {
		return
	}
	for _, old := range former {
		if err := long.Adopt(old, key); err != nil {
			log.Printf("history database: adopting %q as %q: %v", old, key, err)
		}
	}
	if err := long.Import(key, history); err != nil {
		log.Printf("history database: importing %q: %v", key, err)
	}
}

// formerKeys are the history keys with the character's name (bare or with another realm text) that no character in
// the dump uses and whose newest dump has the same class and race.
// shortcut: two characters with one name, class and race on two realms cannot be told apart here, the first to log in
// takes the old history; upgrade when dumps from before the realm number can be matched some other way.
func formerKeys(db *snapshot.Database, key string, history History, files keyedHistory) []string {
	keys, err := files.Keys()
	if err != nil {
		return nil
	}
	name, _, _ := strings.Cut(key, "-")
	class, race := db.Characters[key].ClassAndRace()
	if class == "" || race == "" {
		return nil
	}
	var former []string
	for _, old := range keys {
		if _, live := db.Characters[old]; live || (old != name && !strings.HasPrefix(old, name+"-")) {
			continue
		}
		if c, r := newestClassAndRace(history, old); c == class && r == race {
			former = append(former, old)
		}
	}
	return former
}

func newestClassAndRace(history History, key string) (class, race string) {
	times, err := history.List(key)
	if err != nil || len(times) == 0 {
		return "", ""
	}
	s, err := history.Load(key, times[len(times)-1])
	if err != nil {
		return "", ""
	}
	return s.ClassAndRace()
}
