// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package sharedump turns one character's dump into a copy a player can publish: names of players, guilds and
// pets become aliases, free text the player or other people wrote is removed, and anything that identifies the
// realm is dropped. It works on the raw dump (the boundary), does no I/O and never sees the account name.
//
// Privacy by default: a section the rules do not know is left out, so a new addon section is not shared until
// somebody has decided what is personal in it.
package sharedump

// treatment says what happens to a value the rules point at.
type treatment int

const (
	// playerName is a player character: it gets an alias, and the name is then removed from every other text too.
	playerName treatment = iota + 1
	// guildName works like playerName, with its own aliases.
	guildName
	// correspondent is a name that may be a player or an NPC (a mail sender, an auction owner): an alias in that
	// field only, because a name like "Auction House" must not be cut out of other texts.
	correspondent
	// petName is a name the player may have given a pet; a pet still called after its family keeps the name.
	petName
	// label is a name the player invented for something of their own (a macro, an equipment set).
	label
	// freeText is text written by the player or by other people; it is replaced with removedText.
	freeText
	// drop removes the field.
	drop
)

// removedText stands where free text was, so the field keeps its type.
const removedText = "[removed]"

// rule points at values by their path inside the dump. A path is dot-separated: "*" matches any key and "[]"
// any list item.
type rule struct {
	path  string
	treat treatment
	// unless names a sibling field: when it holds the same text, the value is a default name and stays.
	unless string
	// needsSibling keeps the value when the unless field is missing: without it nothing says the name is custom.
	needsSibling bool
	// alias is the alias prefix for label and petName ("Macro" gives "Macro 1").
	alias string
}

var pathRules = []rule{
	{path: "character.name", treat: playerName},
	{path: "character.guild", treat: guildName},
	{path: "character.title", treat: freeText},
	{path: "friends.friends[].name", treat: playerName},
	{path: "friends.friends[].notes", treat: freeText},
	{path: "guild.name", treat: guildName},
	{path: "guild.motd", treat: freeText},
	{path: "guild.info", treat: freeText},
	{path: "guild.members[].name", treat: playerName},
	{path: "guild.members[].note", treat: freeText},
	{path: "group.leader", treat: playerName},
	{path: "group.members[].name", treat: playerName},
	{path: "inspect.*.name", treat: playerName},
	{path: "inspect.*.guild", treat: guildName},
	{path: "mail.messages[].sender", treat: correspondent},
	{path: "mail.messages[].subject", treat: freeText},
	{path: "note.text", treat: freeText},
	{path: "macros[].name", treat: label, alias: "Macro"},
	{path: "macros[].body", treat: freeText},
	{path: "equipmentSets[].name", treat: label, alias: "Set"},
	{path: "pet.name", treat: petName, unless: "family", alias: "Pet"},
	{path: "stable.pets[].name", treat: petName, unless: "family", alias: "Pet"},
	{path: "collections.companions.list[].name", treat: petName, unless: "species", needsSibling: true, alias: "Pet"},
}

// playerKeyed lists the maps whose keys are player names.
var playerKeyed = map[string]bool{"inspect": true}

// keyRules apply to a field of that name anywhere in the dump: a second net for fields the path rules do not
// list yet (a new addon version, a section that grew).
var keyRules = map[string]treatment{
	"guid": drop, "realm": drop, "realmId": drop, "realmName": drop,
	"officerNote": freeText, "publicNote": freeText, "motd": freeText,
	"sender": correspondent, "owner": correspondent, "seller": correspondent, "bidder": correspondent,
	"guildName": guildName, "playerName": playerName,
}

// knownSections are the sections whose personal fields the rules above cover. Everything else is left out.
var knownSections = map[string]bool{
	"achievements": true, "actionBars": true, "auctions": true, "auras": true, "bags": true, "bank": true,
	"buyback": true, "character": true, "collections": true, "completedQuests": true, "cooldowns": true,
	"currencies": true, "deaths": true, "dungeonLoot": true, "dungeons": true, "equipment": true,
	"equipmentSets": true, "eventLog": true, "flightPaths": true, "friends": true, "group": true, "guide": true,
	"guild": true, "inspect": true, "legacy": true, "lookups": true, "loot": true, "macros": true, "mail": true,
	"note": true, "pet": true, "played": true, "probe": true, "professions": true, "pvp": true, "questDb": true,
	"questLookups": true, "questOffer": true, "questTurnIn": true, "quests": true, "recipes": true,
	"reputation": true, "spells": true, "stable": true, "statistics": true, "stats": true, "talents": true,
	"titles": true, "trainer": true, "trainers": true, "vendor": true, "vendors": true, "xpGains": true,
	"zoneQuests": true,
}

// KnownSection says whether the rules cover a section, so it can be shared.
func KnownSection(name string) bool { return knownSections[name] }
