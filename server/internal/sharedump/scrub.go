// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package sharedump

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// scrubber removes known names from free-running text (quest texts greet the character, a tooltip names the
// crafter). Names match as whole words, in any letter case.
type scrubber struct {
	names []secret
	// realms are matched as plain substrings: a realm name may contain spaces and digits.
	realms []string
}

type secret struct {
	folded []rune
	alias  string
}

const realmAlias = "Realm"

// aliasWords are never treated as names: a character called "Player" would otherwise match its own alias forever.
var aliasWords = map[string]bool{"player": true, "guild": true, "realm": true, "sender": true, "pet": true}

// playerGUID is the GUID of a player character: "Player-<realm number>-<hex>".
var playerGUID = regexp.MustCompile(`Player-\d+-[0-9A-Fa-f]{6,}`)

const guidAlias = "Player-0-00000000"

func newScrubber(secrets map[string]string, realms []string) *scrubber {
	s := &scrubber{}
	for name, alias := range secrets {
		if folded := []rune(name); len(folded) >= 2 && !aliasWords[name] {
			s.names = append(s.names, secret{folded, alias})
		}
	}
	// Longest first, so "Annabel" is not half-replaced as "Anna"; then by name so the order is stable.
	sort.Slice(s.names, func(i, j int) bool {
		a, b := s.names[i].folded, s.names[j].folded
		if len(a) != len(b) {
			return len(a) > len(b)
		}
		return string(a) < string(b)
	})
	for _, realm := range realms {
		if len([]rune(realm)) >= 3 {
			s.realms = append(s.realms, realm)
		}
	}
	return s
}

// clean returns the text without known names and how many places it changed.
func (s *scrubber) clean(text string) (string, int) {
	changed := 0
	if playerGUID.MatchString(text) {
		text = playerGUID.ReplaceAllStringFunc(text, func(string) string { changed++; return guidAlias })
	}
	for _, realm := range s.realms {
		if n := countFold(text, realm); n > 0 {
			text = replaceFold(text, realm, realmAlias)
			changed += n
		}
	}
	for _, name := range s.names {
		var n int
		if text, n = replaceWord(text, name); n > 0 {
			changed += n
		}
	}
	return text, changed
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// replaceWord replaces every whole-word occurrence of the name, comparing rune by rune without letter case.
func replaceWord(text string, name secret) (string, int) {
	runes := []rune(text)
	if len(runes) < len(name.folded) {
		return text, 0
	}
	var out strings.Builder
	count := 0
	for i := 0; i < len(runes); {
		if matchesAt(runes, i, name.folded) {
			out.WriteString(name.alias)
			i += len(name.folded)
			count++
			continue
		}
		out.WriteRune(runes[i])
		i++
	}
	if count == 0 {
		return text, 0
	}
	return out.String(), count
}

func matchesAt(runes []rune, at int, word []rune) bool {
	end := at + len(word)
	if end > len(runes) || (at > 0 && isWordRune(runes[at-1])) || (end < len(runes) && isWordRune(runes[end])) {
		return false
	}
	for i, r := range word {
		if unicode.ToLower(runes[at+i]) != r {
			return false
		}
	}
	return true
}

func countFold(text, part string) int {
	return strings.Count(strings.ToLower(text), strings.ToLower(part))
}

// replaceFold replaces a substring without regard to letter case.
func replaceFold(text, part, with string) string {
	return regexp.MustCompile(`(?i)`+regexp.QuoteMeta(part)).ReplaceAllString(text, with)
}

// tooltipTag is a tooltip line in angle brackets. The game uses them for instructions and for "<Made by Name>",
// which names a player in the client's language; only the known instructions stay.
var tooltipTag = regexp.MustCompile(`<[^<>\n]*>`)

var harmlessTags = map[string]bool{
	"<right click to open>": true, "<right click to read>": true, "<right click for details>": true,
	"<shift right click to socket>": true, "<right click to socket>": true,
	"<shift click to buy a different amount>": true,
}

const removedTag = "<removed>"

func cleanTooltip(text string) (string, int) {
	changed := 0
	text = tooltipTag.ReplaceAllStringFunc(text, func(tag string) string {
		if harmlessTags[strings.ToLower(tag)] || tag == removedTag {
			return tag
		}
		changed++
		return removedTag
	})
	return text, changed
}
