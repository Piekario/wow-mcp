// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package compare puts two characters of the account side by side: level, class, gold,
// play time, professions, equipment slot by slot and the dump age of each.
package compare

import (
	"fmt"
	"strings"

	"wowmcp/internal/snapshot"
)

// Query is the input of wow_compare_characters.
type Query struct {
	A string `json:"a,omitempty" jsonschema:"first character; empty = the most recently dumped character"`
	B string `json:"b,omitempty" jsonschema:"second character; empty = the only other character on the account (when there is just one)"`
}

// Resolve picks two different characters: empty a = the most recently dumped, empty b = the only other character.
// A name can be the full key ("Piekasz-Realm"), a name without the realm or an unambiguous prefix, case-insensitive.
func Resolve(db *snapshot.Database, q Query) (a, b string, err error) {
	if db == nil || len(db.Characters) == 0 {
		return "", "", fmt.Errorf("no characters in the dump. %s", snapshot.AltsHint)
	}
	if len(db.Characters) < 2 {
		return "", "", fmt.Errorf("the account has only one character (%v), nothing to compare with. %s", db.CharacterNames(), snapshot.AltsHint)
	}
	if a, err = find(db, q.A, db.Last); err != nil {
		return "", "", err
	}
	if q.B == "" {
		var others []string
		for _, n := range db.CharacterNames() {
			if n != a {
				others = append(others, n)
			}
		}
		if len(others) != 1 {
			return "", "", fmt.Errorf("specify the second character (b): the account has several. Available: %v", others)
		}
		return a, others[0], nil
	}
	if b, err = find(db, q.B, ""); err != nil {
		return "", "", err
	}
	if a == b {
		return "", "", fmt.Errorf("that is the same character (%s); give two different ones. Available: %v", a, db.CharacterNames())
	}
	return a, b, nil
}

// find matches a name to a character key; empty query = fallback.
func find(db *snapshot.Database, query, fallback string) (string, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		q = fallback
	}
	if _, ok := db.Characters[q]; ok {
		return q, nil
	}
	low := strings.ToLower(q)
	var exact, prefix []string
	for _, n := range db.CharacterNames() {
		ln := strings.ToLower(n)
		short, _, _ := strings.Cut(ln, "-")
		switch {
		case ln == low || short == low:
			exact = append(exact, n)
		case low != "" && strings.HasPrefix(ln, low):
			prefix = append(prefix, n)
		}
	}
	for _, found := range [][]string{exact, prefix} {
		if len(found) == 1 {
			return found[0], nil
		}
		if len(found) > 1 {
			return "", fmt.Errorf("name %q matches several characters: %v. Give the full name", query, found)
		}
	}
	return "", fmt.Errorf("no character %q in the dump. Available: %v", query, db.CharacterNames())
}
