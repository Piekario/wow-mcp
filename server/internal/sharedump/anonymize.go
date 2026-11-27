// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package sharedump

import (
	"fmt"
	"sort"
	"strings"

	"wowmcp/internal/snapshot"
)

// Shared is a dump ready to be published, with the account of what was done to it.
type Shared struct {
	Snapshot snapshot.Snapshot
	Identity Identity
	Report   Report
}

// Identity is what a shared dump still says about its character: enough to know which class and client it covers.
type Identity struct {
	Class, Race, Faction  string
	Level                 int64
	Locale, ClientVersion string
	ClientBuild           string
	AddonVersion          string
	DumpedAt              int64
}

// Report counts what the anonymizer did; it holds no original text.
type Report struct {
	Sections []string
	// LeftOut are the sections this server version has no privacy rules for, so they were not shared.
	LeftOut                               []string
	Players, Guilds, Senders, Pets, Named int
	TextsRemoved, FieldsDropped           int
	NamesScrubbed, TooltipTagsRemoved     int
}

// Anonymize builds the shareable copy of one character's dump. characterKey is the key the dump is stored under
// ("Name", "Name-Realm" or "Name-<realm number>"). The given snapshot is not changed.
func Anonymize(characterKey string, snap snapshot.Snapshot) (Shared, error) {
	a := &anonymizer{aliases: newAliases()}
	out := a.knownSections(snap)
	if len(a.report.Sections) == 0 {
		return Shared{}, fmt.Errorf("the dump has no section that can be shared")
	}
	name, realms := owner(characterKey, snap)
	a.aliases.self(name)

	a.applyRules("", out)
	scrub := newScrubber(a.aliases.secrets(), realms)
	out = a.scrubMap(scrub, out, false)
	if left := residue(scrub, out); left > 0 {
		return Shared{}, fmt.Errorf("%d known names would still be in the shared dump; nothing was written, please report this as a bug", left)
	}
	a.countAliases()
	return Shared{Snapshot: out, Identity: readIdentity(out), Report: a.report}, nil
}

type anonymizer struct {
	aliases *aliases
	report  Report
}

// owner reads the character's name and every form of the realm name the dump knows.
func owner(characterKey string, snap snapshot.Snapshot) (name string, realms []string) {
	character := snap.Character()
	name, _ = character["name"].(string)
	if name == "" {
		name = baseName(characterKey)
	}
	if realm := strings.TrimPrefix(characterKey, name+"-"); realm != characterKey && !isNumber(realm) {
		realms = append(realms, realm)
	}
	for _, field := range []string{"realm", "realmName"} {
		if realm, _ := character[field].(string); realm != "" {
			realms = append(realms, realm)
		}
	}
	return name, realms
}

func isNumber(text string) bool {
	for _, r := range text {
		if r < '0' || r > '9' {
			return false
		}
	}
	return text != ""
}

// knownSections copies the sections the rules cover, with the fields that describe the dump itself.
func (a *anonymizer) knownSections(snap snapshot.Snapshot) snapshot.Snapshot {
	out := snapshot.Snapshot{}
	for name, value := range snap {
		switch {
		case snapshot.IsMetaField(name):
			out[name] = deepCopy(value)
		case knownSections[name]:
			out[name] = deepCopy(value)
			a.report.Sections = append(a.report.Sections, name)
		default:
			a.report.LeftOut = append(a.report.LeftOut, name)
		}
	}
	sort.Strings(a.report.Sections)
	sort.Strings(a.report.LeftOut)
	return out
}

func deepCopy(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = deepCopy(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = deepCopy(item)
		}
		return out
	}
	return value
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// applyRules walks the dump in key order (so aliases are numbered the same way every time) and treats every
// field a rule points at.
func (a *anonymizer) applyRules(path string, node map[string]any) {
	for _, key := range sortedKeys(node) {
		childPath := joinPath(path, key)
		if r, found := ruleFor(childPath, key); found && a.treat(node, key, r) {
			continue
		}
		a.descend(childPath, node[key])
	}
	if playerKeyed[path] {
		a.renamePlayerKeys(node)
	}
}

func (a *anonymizer) descend(path string, value any) {
	switch v := value.(type) {
	case map[string]any:
		a.applyRules(path, v)
	case []any:
		for _, item := range v {
			a.descend(path+"[]", item)
		}
	}
}

func joinPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func ruleFor(path, key string) (rule, bool) {
	for _, r := range pathRules {
		if pathMatches(r.path, path) {
			return r, true
		}
	}
	if treat, found := keyRules[key]; found {
		return rule{treat: treat}, true
	}
	return rule{}, false
}

func pathMatches(pattern, path string) bool {
	want, have := strings.Split(pattern, "."), strings.Split(path, ".")
	if len(want) != len(have) {
		return false
	}
	for i := range want {
		if want[i] != "*" && want[i] != have[i] {
			return false
		}
	}
	return true
}

// treat applies the rule to node[key] and says whether the field is done (false = walk into it as usual).
func (a *anonymizer) treat(node map[string]any, key string, r rule) bool {
	if r.treat == drop {
		delete(node, key)
		a.report.FieldsDropped++
		return true
	}
	text, isText := node[key].(string)
	if !isText {
		return false
	}
	if text == "" || isAlias(text) || r.keepsDefault(node, text) {
		return true
	}
	switch r.treat {
	case playerName:
		node[key] = a.aliases.player(text)
	case guildName:
		node[key] = a.aliases.guild(text)
	case correspondent:
		node[key] = a.aliases.correspondent(text)
	case petName, label:
		node[key] = a.aliases.named(r.alias, text)
	case freeText:
		node[key] = removedText
		a.report.TextsRemoved++
	}
	return true
}

// keepsDefault says the value is not a name the player chose: it equals the sibling field the rule names.
func (r rule) keepsDefault(node map[string]any, text string) bool {
	if r.unless == "" {
		return false
	}
	sibling, present := node[r.unless].(string)
	if !present {
		return r.needsSibling
	}
	return foldName(sibling) == foldName(text)
}

func (a *anonymizer) renamePlayerKeys(node map[string]any) {
	for _, key := range sortedKeys(node) {
		if isAlias(key) {
			continue
		}
		alias := a.aliases.player(key)
		node[alias] = node[key]
		delete(node, key)
	}
}

// scrubMap removes the known names from every text and key left in the dump.
func (a *anonymizer) scrubMap(s *scrubber, node map[string]any, inTooltip bool) map[string]any {
	out := make(map[string]any, len(node))
	for key, value := range node {
		cleanKey, n := s.clean(key)
		a.report.NamesScrubbed += n
		out[cleanKey] = a.scrubValue(s, value, inTooltip || key == "tooltip")
	}
	return out
}

func (a *anonymizer) scrubValue(s *scrubber, value any, inTooltip bool) any {
	switch v := value.(type) {
	case map[string]any:
		return a.scrubMap(s, v, inTooltip)
	case []any:
		for i, item := range v {
			v[i] = a.scrubValue(s, item, inTooltip)
		}
		return v
	case string:
		if inTooltip {
			var tags int
			v, tags = cleanTooltip(v)
			a.report.TooltipTagsRemoved += tags
		}
		clean, n := s.clean(v)
		a.report.NamesScrubbed += n
		return clean
	}
	return value
}

// residue counts the known names still found in the finished copy; anything but zero is a bug in the rules.
func residue(s *scrubber, value any) int {
	switch v := value.(type) {
	case snapshot.Snapshot:
		return residue(s, map[string]any(v))
	case map[string]any:
		total := 0
		for key, item := range v {
			_, n := s.clean(key)
			total += n + residue(s, item)
		}
		return total
	case []any:
		total := 0
		for _, item := range v {
			total += residue(s, item)
		}
		return total
	case string:
		_, n := s.clean(v)
		return n
	}
	return 0
}

func (a *anonymizer) countAliases() {
	a.report.Players = len(a.aliases.players)
	a.report.Guilds = len(a.aliases.guilds)
	for prefix, known := range a.aliases.local {
		switch prefix {
		case "Sender":
			a.report.Senders += len(known)
		case "Pet":
			a.report.Pets += len(known)
		default:
			a.report.Named += len(known)
		}
	}
}

func readIdentity(snap snapshot.Snapshot) Identity {
	c := snap.Character()
	text := func(field string) string { value, _ := c[field].(string); return value }
	id := Identity{
		Class: text("classToken"), Race: text("raceToken"), Faction: text("faction"),
		Locale: text("locale"), ClientVersion: text("clientVersion"), ClientBuild: text("clientBuild"),
	}
	if id.Class == "" {
		id.Class = text("class")
	}
	if id.Race == "" {
		id.Race = text("race")
	}
	id.Level, _ = snap.LevelAndMoney()
	id.AddonVersion, _ = snap["addonVersion"].(string)
	id.DumpedAt, _ = snap.DumpedAt()
	return id
}
