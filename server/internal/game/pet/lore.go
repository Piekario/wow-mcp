// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package pet

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The Forever Beast Lore tooltip format is not confirmed, so the lines are read tolerantly. A part of a line (after
// the last colon, split at commas and semicolons) is an ability rank when it ends with a number and either
//   - has the rank in parentheses: "Bite (Rank 3)", "Bite (3)";
//   - has a rank word before the number: "Bite Rank 3", where the rank words are the first word of the rank texts the
//     client itself wrote elsewhere in the dump ("Rank 3", "Rang 3"), so any client language works;
//   - or has the name of an ability the dump already knows (pet, Beast Training, tamed beasts): "Bite 3".
// Anything else ("Health: 450", "Level 12 Beast") is not an ability.

const maxAbilityName = 40

var parenthesized = regexp.MustCompile(`^(.+?)\s*\(([^()]*)\)$`)

type loreParser struct {
	rankWords map[string]bool // lower case
	known     map[string]bool // lower-case ability names
}

func (p loreParser) parse(lines []string) []ObservedAbility {
	var out []ObservedAbility
	for _, line := range lines {
		text := strings.ReplaceAll(line, " | ", " ")
		if i := strings.LastIndex(text, ":"); i >= 0 {
			text = text[i+1:]
		}
		for _, part := range strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ';' }) {
			if a, ok := p.ability(strings.TrimSpace(part)); ok {
				a.Line = line
				out = append(out, a)
			}
		}
	}
	return out
}

func (p loreParser) ability(part string) (ObservedAbility, bool) {
	if m := parenthesized.FindStringSubmatch(part); m != nil {
		rank := trailingNumber(strings.Fields(m[2]))
		return ObservedAbility{Name: m[1], Rank: rank}, rank > 0 && validName(m[1])
	}
	fields := strings.Fields(part)
	rank := trailingNumber(fields)
	if rank == 0 {
		return ObservedAbility{}, false
	}
	words := fields[:len(fields)-1]
	if len(words) >= 2 && p.rankWords[strings.ToLower(words[len(words)-1])] {
		name := strings.Join(words[:len(words)-1], " ")
		return ObservedAbility{Name: name, Rank: rank}, validName(name)
	}
	name := strings.Join(words, " ")
	return ObservedAbility{Name: name, Rank: rank}, validName(name) && p.known[strings.ToLower(name)]
}

// trailingNumber is the positive whole number in the last field, or 0.
func trailingNumber(fields []string) int {
	if len(fields) == 0 {
		return 0
	}
	n, err := strconv.Atoi(fields[len(fields)-1])
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// validName: starts with a letter, has no digits and is not longer than an ability name gets.
func validName(name string) bool {
	first, _ := utf8.DecodeRuneInString(name)
	if !unicode.IsLetter(first) || utf8.RuneCountInString(name) > maxAbilityName {
		return false
	}
	return !strings.ContainsFunc(name, unicode.IsDigit)
}

// rankWords are the first words of rank texts in the dump ("Rank 3" -> "rank"), in the client's language.
func rankWords(p Pet, lessons Lessons, beasts []Observation, spellRanks []string) map[string]bool {
	texts := append([]string{}, spellRanks...)
	for _, a := range p.Abilities {
		texts = append(texts, a.Rank)
	}
	for _, l := range lessons.List {
		texts = append(texts, l.RankText)
	}
	for _, b := range beasts {
		for _, a := range b.Tamed {
			texts = append(texts, a.RankText)
		}
	}
	words := map[string]bool{}
	for _, t := range texts {
		if f := strings.Fields(t); len(f) == 2 && RankNumber(t) > 0 {
			words[strings.ToLower(f[0])] = true
		}
	}
	return words
}

// abilityNames are the lower-case names of pet abilities the dump knows.
func abilityNames(p Pet, lessons Lessons, beasts []Observation) map[string]bool {
	names := map[string]bool{}
	for _, a := range p.Learned() {
		names[strings.ToLower(a.Name)] = true
	}
	for _, l := range lessons.List {
		names[strings.ToLower(l.Name)] = true
	}
	for _, b := range beasts {
		for _, a := range b.Tamed {
			names[strings.ToLower(a.Name)] = true
		}
	}
	return names
}
