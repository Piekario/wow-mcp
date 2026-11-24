// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package sharedump

import (
	"encoding/json"
	"strings"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

// Every personal value in this dump carries the word SECRET, so one search proves nothing personal survives.
func personalDump() snapshot.Snapshot {
	item := map[string]any{"name": "Rough Copper Bomb", "tooltip": []any{"Rough Copper Bomb", "<Made by SECRETCrafter>", "<Right Click to Open>"}}
	return snapshot.Snapshot{
		"dumpedAt": int64(1794840582), "addonVersion": "0.21.0",
		"character": map[string]any{
			"name": "SECRETHero", "realm": "SECRET Realm 7 PvE", "realmId": int64(6842), "guid": "Player-6842-00SECRET",
			"class": "Warlock", "classToken": "WARLOCK", "race": "Undead", "raceToken": "Scourge", "faction": "Horde",
			"level": int64(31), "locale": "enUS", "clientVersion": "1.60.1", "clientBuild": "71037", "moneyCopper": int64(100000),
		},
		"friends": map[string]any{"friends": []any{
			map[string]any{"name": "SECRETFriend", "notes": "SECRET note about a friend", "class": "Paladin"},
		}},
		"guild": map[string]any{
			"name": "SECRET Guild", "rank": "Member", "motd": "SECRET raid on Friday",
			"members": []any{
				map[string]any{"name": "SECRETMember-SECRETRealm", "note": "SECRET alt of somebody", "officerNote": "SECRET dkp", "class": "Mage"},
				map[string]any{"name": "SECRETFriend", "class": "Paladin"},
			},
		},
		"group": map[string]any{"members": []any{
			map[string]any{"name": "SECRETHero", "class": "Warlock"},
			map[string]any{"name": "SECRETTank", "class": "Warrior"},
		}},
		"inspect": map[string]any{"SECRETTank": map[string]any{"name": "SECRETTank", "guild": "SECRET Other Guild", "class": "Warrior"}},
		"mail": map[string]any{"messages": []any{
			map[string]any{"sender": "SECRETSender", "subject": "SECRET subject", "moneyCopper": int64(5)},
			map[string]any{"sender": "SECRETFriend", "subject": "SECRET hello"},
		}},
		"auctions": map[string]any{"browse": []any{map[string]any{"name": "Copper Ore", "owner": "SECRETSeller"}}},
		"note":     map[string]any{"text": "SECRET plans for tonight", "setAt": int64(1794840000)},
		"macros": []any{
			map[string]any{"name": "SECRETmacro", "body": "/w SECRETFriend inv", "scope": "account"},
		},
		"equipmentSets": []any{map[string]any{"name": "SECRETset", "id": int64(0)}},
		"pet":           map[string]any{"name": "SECRETpet", "family": "Cat", "level": int64(19)},
		"stable": map[string]any{"pets": []any{
			map[string]any{"name": "Bear", "family": "Bear"},
			map[string]any{"name": "SECRETbear", "family": "Bear"},
		}},
		"collections": map[string]any{"companions": map[string]any{"list": []any{
			map[string]any{"name": "SECRETkitty", "species": "Bombay Cat"},
			map[string]any{"name": "Bombay Cat", "species": "Bombay Cat"},
			map[string]any{"name": "Mechanical Squirrel"},
		}}},
		"questTurnIn": map[string]any{"text": "Well done, SECRETHero. Tell secrethero's friends in SECRET Guild.", "npc": "Gryan Stoutmantle"},
		"titles":      map[string]any{"current": "SECRETHero the Explorer"},
		"bags":        []any{map[string]any{"items": []any{item}}},
		"eventLog":    map[string]any{"entries": []any{map[string]any{"kind": "whisper", "playerName": "SECRETWhisperer", "guid": "Player-6842-0ASECRET"}}},
		"loot":        map[string]any{"sources": map[string]any{"SECRETTank": int64(2), "Defias Pillager": int64(3)}},
		"spells":      []any{map[string]any{"name": "Shadow Bolt", "rank": "Rank 4"}},
		"whispers":    map[string]any{"from": "SECRETStranger"},
	}
}

func anonymize(t *testing.T, key string, snap snapshot.Snapshot) (Shared, string) {
	t.Helper()
	shared, err := Anonymize(key, snap)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(shared, "test")
	if err != nil {
		t.Fatal(err)
	}
	return shared, string(data)
}

func TestNothingPersonalSurvives(t *testing.T) {
	shared, text := anonymize(t, "SECRETHero-SECRET Realm 7 PvE", personalDump())
	if at := strings.Index(strings.ToLower(text), "secret"); at >= 0 {
		t.Errorf("personal text survived: ...%s...", text[max(0, at-80):min(len(text), at+80)])
	}
	for _, gone := range []string{"6842", "Realm 7"} {
		if strings.Contains(text, gone) {
			t.Errorf("%q survived", gone)
		}
	}
	if got := shared.Report.LeftOut; len(got) != 1 || got[0] != "whispers" {
		t.Errorf("an unknown section must be left out and reported: %v", got)
	}
	if _, kept := shared.Snapshot["whispers"]; kept {
		t.Error("the unknown section is in the shared dump")
	}
}

func TestGameDataAndStructureStay(t *testing.T) {
	shared, text := anonymize(t, "SECRETHero-6842", personalDump())
	fixtures.WantAll(t, "shared dump", text,
		`"Shadow Bolt"`, `"Gryan Stoutmantle"`, `"Defias Pillager"`, `"Copper Ore"`, `"Bombay Cat"`, `"Mechanical Squirrel"`,
		`<Right Click to Open>`, `<removed>`, `"Well done, Player. Tell Player's friends in Guild 1."`, `"Player the Explorer"`)
	id := shared.Identity
	if id.Class != "WARLOCK" || id.Race != "Scourge" || id.Level != 31 || id.Locale != "enUS" || id.ClientVersion != "1.60.1" {
		t.Errorf("identity: %+v", id)
	}
	character := shared.Snapshot.Character()
	if character["name"] != "Player" || character["moneyCopper"] != int64(100000) {
		t.Errorf("character: %v", character)
	}
	for _, dropped := range []string{"realm", "realmId", "guid"} {
		if _, kept := character[dropped]; kept {
			t.Errorf("character.%s was kept", dropped)
		}
	}
	stable := snapshot.AsList(shared.Snapshot.Map("stable")["pets"])
	if snapshot.AsMap(stable[0])["name"] != "Bear" || snapshot.AsMap(stable[1])["name"] == "Bear" {
		t.Errorf("a pet named after its family keeps the name, a custom name does not: %v", stable)
	}
}

// The same player gets the same alias everywhere, so a group member still matches their inspect entry.
func TestAliasesAreConsistent(t *testing.T) {
	shared, _ := anonymize(t, "SECRETHero", personalDump())
	members := snapshot.AsList(shared.Snapshot.Map("group")["members"])
	tank, _ := snapshot.AsMap(members[1])["name"].(string)
	inspected := snapshot.AsMap(shared.Snapshot.Map("inspect")[tank])
	if tank == "" || inspected["name"] != tank {
		t.Errorf("group member %q has no inspect entry under the same alias: %v", tank, shared.Snapshot.Map("inspect"))
	}
	if snapshot.AsMap(members[0])["name"] != "Player" {
		t.Errorf("the owner in the group: %v", members[0])
	}
	messages := snapshot.AsList(shared.Snapshot.Map("mail")["messages"])
	friend := snapshot.AsMap(snapshot.AsList(shared.Snapshot.Map("friends")["friends"])[0])["name"]
	if snapshot.AsMap(messages[1])["sender"] != friend {
		t.Errorf("mail from a friend keeps the friend's alias: %v vs %v", messages[1], friend)
	}
	if sender, _ := snapshot.AsMap(messages[0])["sender"].(string); !strings.HasPrefix(sender, "Sender ") {
		t.Errorf("an unknown sender: %q", sender)
	}
}

func TestSourceDumpIsNotChanged(t *testing.T) {
	snap := personalDump()
	before := fixtures.JSON(t, snap)
	anonymize(t, "SECRETHero", snap)
	if fixtures.JSON(t, snap) != before {
		t.Error("Anonymize changed the dump it was given")
	}
}

func TestSharingASharedDumpChangesNothing(t *testing.T) {
	first, text := anonymize(t, "SECRETHero-6842", personalDump())
	_, again := anonymize(t, "Player", first.Snapshot)
	var a, b map[string]any
	if err := json.Unmarshal([]byte(text), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(again), &b); err != nil {
		t.Fatal(err)
	}
	if fixtures.JSON(t, a["snapshot"]) != fixtures.JSON(t, b["snapshot"]) {
		t.Error("anonymizing an anonymized dump changed it")
	}
}

// A name is replaced as a whole word only: a character called Ash must not damage Ashenvale.
func TestNamesMatchWholeWords(t *testing.T) {
	s := newScrubber(map[string]string{"ash": "Player", "żółć": "Player 2"}, []string{"Realm 7 PvE"})
	for text, want := range map[string]string{
		"Ashenvale":                   "Ashenvale",
		"Hello, ASH!":                 "Hello, Player!",
		"Ash's bag":                   "Player's bag",
		"Żółć says hi":                "Player 2 says hi",
		"Ash-CAMWOW Realm 7 PvE":      "Player-CAMWOW Realm",
		"Player-6842-0ABCDEF1 dueled": guidAlias + " dueled",
		"Rank 7":                      "Rank 7",
	} {
		if got, _ := s.clean(text); got != want {
			t.Errorf("clean(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestFileNameCannotLeaveTheDirectory(t *testing.T) {
	name := FileName(Identity{Class: "../WAR/LOCK", Level: 31, Locale: `en\US`, DumpedAt: 1794840582})
	if name != "wow-mcp-dump-warlock-31-enUS-20261116-144942.json" {
		t.Errorf("file name: %q", name)
	}
	if got := FileName(Identity{}); got != "wow-mcp-dump-unknown-0.json" {
		t.Errorf("empty identity: %q", got)
	}
}

func TestEmptyDumpIsRefused(t *testing.T) {
	if _, err := Anonymize("X", snapshot.Snapshot{"dumpedAt": int64(1), "whispers": "x"}); err == nil {
		t.Error("a dump without a known section was accepted")
	}
}

// The test dumps of the repository: every section they hold is covered by the rules, and no name of a player,
// guild or realm that the dump names in a personal field is found anywhere in the shared copy.
func TestRepositoryDumps(t *testing.T) {
	dumps := map[string]snapshot.Snapshot{
		"modern": fixtures.Modern(t), "forever": fixtures.Forever(t), "dede": fixtures.Dede(t),
		"real": fixtures.Real(t), "real 0.11.0": fixtures.Real0110(t),
	}
	for name, snap := range dumps {
		shared, text := anonymize(t, fixtures.RealCharacter, snap)
		if len(shared.Report.LeftOut) > 0 {
			t.Errorf("%s: sections without privacy rules: %v (add them to rules.go)", name, shared.Report.LeftOut)
		}
		for _, personal := range []string{"Piekasz", "CAMWOW", "Realm 7", "Tankadin", "Healbot", "Oldfriend", "Night Watch", "dungeon buddy", "I want Deadmines today", "/petattack"} {
			if strings.Contains(text, personal) {
				t.Errorf("%s: %q survived", name, personal)
			}
		}
	}
}
