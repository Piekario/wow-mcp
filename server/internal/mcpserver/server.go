// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package mcpserver is the MCP adapter: registration of tools and prompts, mapping
// of arguments to domain queries and writing responses (JSON with a WARNING line for stale
// data). Game logic lives in the internal/game packages; here we only wire them together.
package mcpserver

import (
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"wowmcp/internal/game/goals"
	"wowmcp/internal/game/items"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// Snapshots is the snapshot source: it reloads the SavedVariables file on every call.
type Snapshots interface {
	Load() (*snapshot.Database, error)
}

// Config holds the server dependencies, wired in main.
type Config struct {
	Name, Version string
	Snapshots     Snapshots
	History       progress.History // nil = history disabled
	HistoryDir    string           // shown in wow_status; "" when history is disabled
	Freshness     snapshot.Freshness
	Goals         goals.Store        // nil = goals disabled (wow_goals and wow_goal_set return an error)
	GoalsFile     string             // shown in wow_status
	Weights       items.WeightsStore // nil = no weights file (wow_compare_item)
	// MaxResponseKB caps the size of one tool answer (flag --max-response-kb); bigger answers become a compact
	// envelope with a narrowing hint. 0 = no cap.
	MaxResponseKB int
}

const instructions = "The tools read World of Warcraft: Forever character state saved by the WowMcp addon. " +
	"The data refreshes only after /mcp reload in game, so check snapshot.ageMinutes and ask " +
	"for a refresh when it is old (the tools then add a WARNING line themselves). " +
	"If a tool errors, returns nothing or the data looks wrong, call wow_doctor (or wow_status) first. " +
	"An answer over the size limit comes back as {truncated: true, hint, topLevel, partial}: follow the hint and ask for less. " +
	"Tools: " +
	"status and version: wow_status (also alerts: mail, auctions, gear, bags and which account was read), wow_doctor (the first thing to call when something looks wrong: ordered checks with fixes), wow_section (any section, big ones in pieces with path and limit); " +
	"character: wow_character (with the player's note and deaths), wow_equipment, wow_bags, wow_spells, wow_talents, wow_legacy (Legacy perks, points and challenges), " +
	"wow_reputation, wow_pet (also missing ranks, training point plan and stable), wow_quests; " +
	"windows from the last visit: wow_trainer and wow_vendor (optionally npc: up to 10 remembered), wow_bank, wow_mail, wow_auctions, wow_recipes; " +
	"shopping and quests: wow_shopping_list (trainer, repairs, ammo, food, who sells it, when to return to town), wow_quest_rewards (which quest reward to pick); " +
	"planning and comparisons: wow_quest_plan, wow_dungeons, wow_group, wow_compare_item, wow_cooldowns; " +
	"progress: wow_changes (what changed since the previous snapshot), wow_progress (XP and gold pace, time to level, deaths in the session), " +
	"wow_gold_history, wow_rested, wow_ammo, wow_reagents (stocks of ammo, food, bandages, potions and their consumption), " +
	"wow_journal (session journal and records, optionally Markdown), wow_timeline (timeline: level-ups, skills, gear, quests, deaths), wow_session_rating (pace of the last session versus the median); " +
	"goals: wow_goals (goal progress and ETA), wow_goal_set (the only tool that writes: adds or removes a goal in the server's goals file); " +
	"bags: wow_bag_plan (what to do with full bags: keep, bank, sell, auction; destroys nothing); " +
	"account and search: wow_account, wow_find_item, wow_search (text across one character's whole snapshot), wow_leftovers (leftover quest items), wow_compare_characters (two characters side by side), wow_alt_mail (what to mail between characters), wow_account_crafting (what the account can craft from combined materials), wow_make_or_buy (craft or buy on the auction house); " +
	"auras and flight: wow_buffs (buffs, debuffs, missing class buffs), wow_flights (flight points versus quest zones), wow_quest_route (suggested order of zones with quests); quest locations and guide: wow_quest_lookup (where a quest starts and ends, from the quest database installed in the game; always use it instead of answering quest locations from memory), wow_guide (next steps of the player's Guidelime / TUGs leveling guide); friends and guild: wow_social; " +
	"wow_export (a summary to paste elsewhere). " +
	"Ready-made prompts: session_plan, trainer_shopping, what_to_sell, gear_upgrades, session_summary, account_overview, " +
	"profession_progress, talent_planning, what_to_auction, dungeon_finder, before_logout, first_run, pet_care, week_in_game. " +
	"Answer in the user's language and keep in-game names (items, spells, zones) in English. " +
	"The server performs no actions in the game; it writes only the snapshot history and the goals file in its own directory (wow_goal_set)."

func New(cfg Config) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: cfg.Name, Version: cfg.Version}, &mcp.ServerOptions{
		Instructions: instructions,
	})
	svc := &service{Config: cfg, progress: progress.Tracker{History: cfg.History}}
	limiter := responseCap{limitKB: cfg.MaxResponseKB, args: map[string][]string{}}
	for _, t := range svc.tools() {
		t.register(server)
		limiter.args[t.name] = t.args
	}
	server.AddReceivingMiddleware(limiter.middleware)
	svc.registerResources(server)
	for _, p := range prompts {
		p.register(server, svc)
	}
	return server
}

type service struct {
	Config
	progress progress.Tracker
	// goalsMu serializes wow_goal_set: adding or removing a goal reads, changes and rewrites the whole goals file,
	// and MCP requests may arrive concurrently (two adds at once would otherwise lose one goal).
	goalsMu sync.Mutex
}

// load reads the file and returns the snapshot of the given character (empty name = most recently dumped).
func (s *service) load(character string) (string, snapshot.Snapshot, error) {
	db, err := s.Snapshots.Load()
	if err != nil {
		return "", nil, err
	}
	return db.Lookup(character)
}
