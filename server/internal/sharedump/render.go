// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package sharedump

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"wowmcp/internal/snapshot"
)

const (
	// Format marks a file written by this package.
	Format        = "wow-mcp-shared-dump"
	FormatVersion = 1
	// IssueURL opens the form a shared dump is attached to.
	IssueURL = "https://github.com/Piekario/wow-mcp/issues/new?template=share_dump.yml"
)

// Removed and Kept say in words what the rules do; the tool answer, the command line and the file itself repeat them.
var (
	Removed = []string{
		"character name, realm name and number, player GUIDs",
		"guild name and the names and notes of its members",
		"friends and their notes, group members, inspected players",
		"mail senders and subjects",
		"the note for the assistant, macro names and bodies, equipment set names, custom pet names",
		"the same names wherever else they appear (quest texts, the crafter line of a made item)",
		"sections this server version has no privacy rules for",
	}
	Kept = []string{
		"class, race, faction, level, client version, build and language",
		"the structure and game data of every shared section: items, spells, talents, quests, professions, reputations, gold, zone",
		"the /mcp probe result (names of client API functions) when the dump has one",
	}
)

// Encode writes the shareable file: a JSON envelope with the report first and the anonymized dump under "snapshot".
func Encode(s Shared, serverVersion string) ([]byte, error) {
	envelope := map[string]any{
		"format": Format, "formatVersion": FormatVersion, "serverVersion": serverVersion,
		"about":     "An anonymized WowMcp dump shared to help wow-mcp support more classes, clients and languages. Read it before you publish it.",
		"character": s.Identity.render(),
		"report":    s.Report.render(),
		"snapshot":  map[string]any(s.Snapshot),
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	if err := enc.Encode(envelope); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// FileName names the file after what the dump covers, e.g. "wow-mcp-dump-warlock-31-enUS-20261124-161500.json".
func FileName(id Identity) string {
	parts := []string{"wow-mcp-dump", strings.ToLower(fileWord(id.Class, "unknown")), fmt.Sprint(id.Level), fileWord(id.Locale, "")}
	if id.DumpedAt > 0 {
		parts = append(parts, time.Unix(id.DumpedAt, 0).UTC().Format("20060102-150405"))
	}
	name := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		if name != "" {
			name += "-"
		}
		name += part
	}
	return name + ".json"
}

// fileWord keeps only letters and digits, so a value from the dump cannot leave the target directory.
func fileWord(value, fallback string) string {
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return fallback
	}
	return b.String()
}

// Export anonymizes one character's dump and returns the file to write: its name and contents.
func Export(characterKey string, snap snapshot.Snapshot, serverVersion string) (Shared, string, []byte, error) {
	shared, err := Anonymize(characterKey, snap)
	if err != nil {
		return Shared{}, "", nil, err
	}
	data, err := Encode(shared, serverVersion)
	if err != nil {
		return Shared{}, "", nil, err
	}
	return shared, FileName(shared.Identity), data, nil
}

func (id Identity) render() map[string]any {
	out := map[string]any{"level": id.Level}
	for key, value := range map[string]string{
		"class": id.Class, "race": id.Race, "faction": id.Faction, "locale": id.Locale,
		"clientVersion": id.ClientVersion, "clientBuild": id.ClientBuild, "addonVersion": id.AddonVersion,
	} {
		if value != "" {
			out[key] = value
		}
	}
	return out
}

func (r Report) render() map[string]any {
	out := map[string]any{
		"sections": r.Sections,
		"replacedWithAliases": map[string]any{
			"players": r.Players, "guilds": r.Guilds, "mailSendersAndAuctionOwners": r.Senders,
			"pets": r.Pets, "macrosAndEquipmentSets": r.Named,
		},
		"textsRemoved":       r.TextsRemoved,
		"fieldsDropped":      r.FieldsDropped,
		"namesFoundInText":   r.NamesScrubbed,
		"tooltipTagsRemoved": r.TooltipTagsRemoved,
	}
	if len(r.LeftOut) > 0 {
		out["sectionsLeftOut"] = r.LeftOut
	}
	return out
}

// Answer is what the tool and the command line say after the file was written.
func Answer(s Shared, path string) map[string]any {
	return map[string]any{
		"file":      path,
		"character": s.Identity.render(),
		"report":    s.Report.render(),
		"removed":   Removed,
		"kept":      Kept,
		"nextSteps": []string{
			"Open the file and read it: it is plain JSON on your own disk, and nothing has been sent anywhere.",
			"If anything in it still looks personal, do not publish it; say what it was in the issue instead, so the rule can be fixed.",
			"To share it, attach the file to a new issue: " + IssueURL,
		},
		"note": "The server only wrote this file in its own directory. It made no network connection and never uploads anything; publishing the file is the player's own step.",
	}
}
