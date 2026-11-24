# wow-mcp roadmap

As of 2026-11-16: WowMcp addon 0.12.0, wow-mcp server 0.13.0 (verified in the game so far: addon 0.11.0 with server 0.12.2, 2026-11-16) (changes in `CHANGELOG.md`). Addon 0.5.0 and server 0.6.0
were verified in a real Forever 1.60.1 client that day (zero section errors, dump 18 ms), and the real dump is
now in the tests (the `real` scenario). **Addon 0.6.0 / 0.7.0 and server 0.7.0 / 0.8.0 / 0.9.0 had not yet been run in the game.**
So what remains open is deploying and verifying the new things in the client: quest rewards by ID, quest windows at NPCs, the stable, loot,
the tooltip price, the auction house, bank, mail and recipes, and the addon 0.7.0 sections (aura duration, flight point zone,
`xpGains`, `buyback`, `equipmentSets`, `friends`, `guild`, `pvp`, the minimap button, the compact panel mode).
The API names for the new sections are candidates to be confirmed with `/mcp probe`.

**Marks:** ✅ done · ◐ partly done.

**Rule for every item:** the addon only reads through the official API and does nothing without a player click, the server reads
only `WowMcp.lua` and writes only in its own directory. Ideas that would break this are at the end, in the
"Rejected" section.

**Priorities:** 🔴 now · 🟡 soon · ⚪ someday
**Size:** S = up to 2 hours · M = half a day to a day · L = several days

---

## Where to start (🔴 now)

| # | Idea | Size |
|---|---|---|
| 1 | Deploy addon 0.12.0 and server 0.13.0 on the computer with the game (`install.ps1`, owner) and go through the list in `docs/FIRST_RUN.md` | S |
| 4 | Open the mailbox while a letter with gold or an attachment waits, then `/mcp reload`: the auction house, bank and recipes were checked on real data on 2026-11-24, `mail` was only ever saved from an empty mailbox | S |
| 16–18, 23, 37 | Check in the game the 0.6.0 data that was not in the 0.7.0 dump: quest rewards (whether the client respects the ID, the `suspect` flag), quest windows at NPCs, loot (`loot.messagesParsed`), the stable (`C_StableInfo`), the tooltip price | S |
| 20, 25, 28, 29, 41, 101, 102 | Check in the game the 0.7.1 fixes: `xpGains` (`messagesParsed`, `barXp`), the buyback list and `vendors` / `trainers`, friends (after opening the window), flight point zone (`zoneSources`), the minimap button and `/mcp compact`, keeping the dump on exiting the game. Confirmed in 0.7.0: aura `expiresAt`, `pvp` (without rank), `equipmentSets`, talent descriptions; `guild` empty because the character is not in a guild (0.7.1 writes `inGuild = false`) | S |

### Candidates for the next tasks (🟡, after deployment)

| # | Idea | Why now | Size |
|---|---|---|---|
| 69, 70, 105 | Check in the game the addon 0.22.0 pet data and class buff IDs: `/mcp probe` with food in the bags (`probe.petFood`: which argument form of `C_PetInfo.CanPetEatItem` answers), Beast Lore on a targeted beast (`probe.beastLore`: the tooltip format), and one taming (`beasts`); `/mcp reload`, then `wow_doctor` (check `class buff spell IDs`) and `wow_buffs` (`groupIdMismatches`) | the signature, the events and the tooltip format are assumptions, checked only on mocks | S |

---

## 1. Deployment and in-game verification

| # | Idea | What it is | Size | Priority |
|---|---|---|---|---|
| 1 | ✅ Deploying 0.5.0 / 0.6.0 | Run `install.ps1` (addon to `AddOns`, `bin\wow-mcp.new.exe` in place of `wow-mcp.exe`), restart the MCP client, `/mcp reload`. Done for 0.5.0 / 0.6.0 (verified in the client on 2026-11-15, zero section errors, dump 18 ms). Addon 0.7.0 and server 0.9.0 deployed and run on 2026-11-15 (zero section errors, dump 29 ms). Addon 0.7.1 and server 0.9.1 await deployment and verification per `docs/FIRST_RUN.md`. | S | 🔴 |
| 2 | ✅ PowerShell installer | `install.ps1` copies the addon, renames the running exe to `.old`, copies the new one and finally runs `--check`. Done: `install.ps1` (`-WhatIf`, `-Build`, `-SkipCheck`). | M | 🔴 |
| 3 | ✅ First-run checklist | A short file with the steps `/mcp reload`, `/mcp status`, `/mcp probe`, `wow_status` and what to paste to the assistant when there are errors. Done: `docs/FIRST_RUN.md`. | S | 🔴 |
| 4 | ◐ Auction house verification | Check whether Forever uses the classic `GetAuctionItemInfo` or `C_AuctionHouse`, and fix the `auctions` section. Partly: `/mcp probe` (2026-11-15) showed only `C_AuctionHouse`, and the `BROWSE_RESULTS_*` events are rejected; addon 0.7.1 listens to `AUCTION_HOUSE_BROWSE_RESULTS_*`. Checked in the real client on 2026-11-24 (addon 0.20.0, no section errors): `auctions` after searches (prices, 25 own auctions with sold flags), `bank` (9 items, slot counts) and `recipes` all carry real data. The recipes check found a window whose `professionName` is empty (one recipe, Camp Chair), saved under the key `""`: fixed after 0.20.0 (saved as `Unnamed profession` with the client's fields in `windowInfo`). Still to check: `mail` was only ever saved from an empty mailbox, so a letter with gold or an attachment has not been read in the real client yet. | S | 🔴 |
| 5 | ✅ Dungeon verification with LFG | Check whether `C_LFGList` gives level ranges; if not, the classic table remains the main source. Verified in a real client: `C_LFGList` gives level ranges, also for dungeons that exist only in Forever; the classic table remains as a fallback. | S | 🔴 |
| 6 | ✅ Talent description verification | Check whether `C_Spell.GetSpellDescription` returns descriptions of `C_Traits` nodes. Verified: in the addon 0.7.0 dump from a real client all nodes have a description (the fallback from the spell tooltip and `talents.missingDescriptions` remain in case of gaps). | S | 🔴 |
| 7 | ✅ Real dump in tests | An anonymized copy of the dump from the game as `testdata/WowMcp_real.lua`, so server tests run on real data. Done: `server/testdata/real/snapshot_2026-11-15_addon070.json` (addon 0.7.0; earlier the 0.5.0 dump), the `/mcp probe` result and the "empty" dump from exiting the game, `fixtures.Real` / `fixtures.RealLua` and the `real` scenario in golden tests. The path differs from the original description (JSON instead of `.lua`). | M | 🔴 |
| 8 | ✅ Version on top | `wow-mcp.exe --version` and the server and addon version in `wow_status`, so it is immediately visible what is really running. Done: `--version` existed before; `serverVersion` and `addonVersion` in `wow_status` came in 0.6.0. | S | 🔴 |
| 9 | ✅ Restoring the previous version | An `install.ps1 -Rollback` option that returns to the last `.old.exe` and the previous addon copy. Done: `-Rollback` (addon from `backup\` and `.old.exe`). | S | 🟡 |
| 10 | ✅ Cleaning up `bin/` | The installer keeps only the two latest `.old.exe` instead of all of them. Done: `-KeepOld` (default 2). | S | 🟡 |
| 11 | ✅ Interface number check | The installer compares `## Interface:` in `.toc` with the client version and warns about "out of date". Done: a check in `install.ps1`, it only warns. | S | 🟡 |
| 12 | ✅ Entry in the client configuration | The installer adds the `wow` server to the client configuration file (with a backup of the file) if it is not there. | M | 🟡 |
| 13 | ✅ Detecting the Store version | The installer finds the configuration on its own, also in the Microsoft Store version of the client. | S | ⚪ |

## 2. Addon: new data (read-only)

| # | Idea | What it is | Size | Priority |
|---|---|---|---|---|
| 14 | ✅ Map position | Zone, subzone and coordinates from `C_Map` at the time of the dump, so the session plan knows where you are. Existed before: zone, subzone and coordinates in `character`. | S | 🟡 |
| 15 | ✅ Hearthstone location | `GetBindLocation()`: where the Hearthstone returns to. Done: `character.bindLocation` (replaced `hearthstone`) and `character.resting`. | S | 🟡 |
| 16 | ✅ Choice rewards | Quest rewards from the log (`GetQuestLogChoiceInfo`), so the assistant can advise which one to take. Done in addon 0.6.0: `quests[].rewards` (choice, fixed items, gold, XP, currencies), read by quest ID without selecting the entry (new API) or with the selection restored (old API). To check in the game whether the client respects the ID (the server detects suspicious repeats: `suspect`). | M | 🟡 |
| 17 | ✅ Quests from NPCs | The text and rewards of a quest saved when the quest window is opened (`QUEST_DETAIL`), before you accept it. Done in addon 0.6.0: the sections `questOffer` (QUEST_DETAIL) and `questTurnIn` (QUEST_COMPLETE) with title, text, NPC and rewards. To check in the client. | M | 🟡 |
| 18 | ✅ Loot since the last dump | Your own loot from `CHAT_MSG_LOOT` since the previous dump (entry limit). Done in addon 0.6.0: the `loot` section from messages about your own loot since the previous write to disk (up to 100 items). To check in the client. | M | 🟡 |
| 19 | ✅ Deaths | A death counter from `PLAYER_DEAD` and the zone where you fell. Done: the `deaths` section (counter and up to 20 recent), visible in `wow_character`, `wow_changes`, `wow_progress`. | S | 🟡 |
| 20 | ✅ XP from monsters and from quests | A split of earned XP into monsters and quests from XP messages. Done: the `xpGains` section (addon 0.7.0: `kills`, `quests`, `other`, quest XP counted once) and `xpSources` in `wow_changes`, `wow_progress` and `wow_journal` (server 0.9.0, package `game/xpgains`). | M | ⚪ |
| 21 | ✅ Multiple vendors | Remember the offer of several recent vendors (e.g. 10), not only the last one. Done in addon 0.6.0: `vendors` (up to 10 recent); `wow_vendor` has the `npc` argument and the `known` field. | M | 🟡 |
| 22 | ✅ Multiple trainers | A separate offer for the class, profession and weapon trainer. Done in addon 0.6.0: `trainers` (up to 10 recent); `wow_trainer` has the `npc` argument and the `known` field. | M | 🟡 |
| 23 | ✅ Stable | Pets in the stable saved when the stable window is opened. Done in addon 0.6.0: the `stable` section (slot, name, level, family). To check in the client. | S | 🟡 |
| 24 | ✅ Pet diet | `GetPetFoodTypes()`, that is, what the pet eats; needed for item 69. Existed before: `pet.foodTypes`. | S | 🟡 |
| 25 | ✅ Buyback at the vendor | The buyback list, so the assistant can remind about an accidentally sold item. Done: the `buyback` section (addon 0.7.0) and the `buyback` field in `wow_vendor` (server 0.9.0); the buyback list is lost on logout, the response marks this as an assumption. No alert was added: `buyback` entries have no item quality. | S | ⚪ |
| 26 | ✅ Equipment sets | Sets from the equipment manager, if the client has them. Done: the `equipmentSets` section (addon 0.7.0) visible in `wow_equipment` (server 0.9.0). | S | ⚪ |
| 27 | ✅ Achievements | An `achievements` section, if Forever has achievements. Done in addon 0.21.0, server 0.21.0 (not yet run in the real client): the `achievements` section (points, categories with counts, up to 600 achievements with completion dates, criteria of incomplete ones; completed ones and the category layout read once per session) and `wow_achievements` (views `summary`, `timeline`, `recent`, `closest`, `all`; package `game/achievements`). The categories are read by one helper shared with the Legacy challenges. API names are assumptions to confirm with `/mcp probe`. | M | ⚪ |
| 124 | ✅ Statistics | The counters the game keeps (Statistics tab). Done in addon 0.21.0, server 0.21.0 (not yet run in the real client): the `statistics` section (`GetStatisticsCategoryList`, `GetStatistic`; up to 400, `--` skipped) and `wow_statistics`. API to confirm with `/mcp probe`. | S | ⚪ |
| 125 | ✅ Titles and collections | Earned titles and the shown one, known mounts and companion pets. Done in addon 0.21.0, server 0.21.0 (not yet run in the real client): the `titles` and `collections` sections (`GetNumTitles` / `IsTitleKnown` / `GetTitleName` / `GetCurrentTitle`; `C_MountJournal` or the companion API, `C_PetJournal` for pets as a fallback) and `wow_collections` (package `game/collections`). Read only: the title is never changed, nothing is summoned. API to confirm with `/mcp probe`. | S | ⚪ |
| 28 | ✅ Honor and PvP | Rank, honor and PvP statistics, if available. Done: the `pvp` section (addon 0.7.0) visible in `wow_character` (server 0.9.0). API names to check in the game (`/mcp probe`). | S | ⚪ |
| 29 | ✅ Guild | The guild member list when the guild window is opened (name, level, class, online). Done: the `guild` section (addon 0.7.0; client events, no `GuildRoster`) and `wow_social` (server 0.9.0, package `game/social`). To check in the game. | M | ⚪ |
| 30 | ✅ Friends | The friends list when the friends window is opened. Done: the `friends` section (addon 0.7.0; no list request to the game server) and `wow_social` (server 0.9.0). To check in the game. | S | ⚪ |
| 31 | Checked, deliberately not doing: smaller dump | Removing repeated item descriptions (a description dictionary), so the file grows more slowly. Measured on the real dump: repeated item detail is 3.5% of the file, not worth a schema change. | M | 🟡 |
| 32 | ✅ Turning off sections | `/mcp off mail` and similar, for privacy or file size. Done: `/mcp off|on <section>`; `character` cannot be turned off. | S | 🟡 |
| 33 | ✅ Event probe | `/mcp probe` also records which events the client knows, to make choosing `RegisterEvent` easier. Done: `probe.events`. | S | ⚪ |
| 101 | ✅ Aura duration | An `expiresAt` (or `remaining`) field in `auras`, from `C_UnitAuras` / `UnitAura`, so `wow_buffs` can say how much time is left on a buff. The server already reads both fields when they are in the dump. For addon 0.7.0. Done: `expiresAt` and `duration` in `auras` (addon 0.7.0); `wow_buffs` computes the time remaining. API to check in the game. | S | 🟡 |
| 102 | ✅ Flight point zone | A `zone` field in the `flightPaths.nodes` entries, so `wow_flights` does not guess the zone from the point's name; while at it, check in the client what the `unreachable` state means. For addon 0.7.0. Done: the `zone` field in `flightPaths.nodes` (addon 0.7.0, from `C_Map.GetMapInfoAtPosition`); `wow_flights` uses it before guessing from the name. The meaning of the `unreachable` state is still to check in the game. | S | 🟡 |

## 3. Addon: in-game interface (display and clicks only)

| # | Idea | What it is | Size | Priority |
|---|---|---|---|---|
| 34 | ✅ Save reminder | The panel highlights the "Save" button when a trainer or the auction house has been opened since the dump. It saves nothing on its own. Done: the panel highlights "Save", never saves on its own. | S | 🟡 |
| 35 | ✅ Errors in the tooltip | Hovering over the panel shows the full list of section errors. Done: the panel tooltip shows section errors. | S | 🟡 |
| 36 | ✅ Note for the assistant | `/mcp note I want Deadmines today` saves a note in the dump, which prompts read as the session goal. Done: `/mcp note`; the note is in `wow_character` and `wow_status`. | S | 🟡 |
| 37 | ✅ Price in the item tooltip | The last saved auction house price in the item tooltip (display only). Done in addon 0.6.0: an "AH: price (age)" line in the item tooltip (`AuctionTooltip.lua`), display only, `/mcp ahtip on|off`. To check in the client. | M | 🟡 |
| 38 | ✅ Settings in the game menu | An interface options page instead of only `/mcp` commands. Done in addon 0.11.0: `Options.lua`, a page under Interface / AddOns with the same switches as `/mcp` (sections, minimap button, compact panel, AH tooltip, played time), built on the existing setting functions; modern `Settings` API and old `InterfaceOptions_AddCategory`, all in `pcall`; `/mcp options` opens it. Mocks only; to check in the client. | M | ⚪ |
| 39 | ✅ Minimap button | A small button that does the same as "Save" and shows the panel. Done: `MinimapButton.lua` (addon 0.7.0): a left click toggles the panel, saves nothing; `/mcp minimap on|off`. | S | ⚪ |
| 40 | ✅ Messages in English | Superseded: since addon 0.8.0 / server 0.10.0 the whole product is English only (no language switch). | S | ⚪ |
| 41 | ✅ Compact panel mode | The panel in a single line, to stick under the minimap. Done: `/mcp compact on|off` (addon 0.7.0). | S | ⚪ |

## 4. Server: new tools

| # | Idea | What it is | Size | Priority |
|---|---|---|---|---|
| 42 | ✅ `wow_search` | Text search across the whole dump (quests, items, abilities, mail) with a single query. Done. | S | 🟡 |
| 43 | ✅ `wow_rested` | Rested XP: how much there is, how many killed monsters that is and when it will be full when logging out in an inn. Done: Classic rules, the response marks them as approximate. | S | 🟡 |
| 44 | ✅ `wow_gold_history` | Gold over time from the dump history: day by day, the largest income and expenses. Done. | S | 🟡 |
| 45 | ✅ `wow_bag_plan` | What to do with full bags: what to put in the bank, what to sell, what to keep for quests and professions. Done: `wow_bag_plan` (keep, bank, sell, auction with a reason; destroys nothing). | M | 🟡 |
| 46 | ✅ `wow_shopping_list` | A shopping list before leaving town: trainer, ammo, pet food, reagents, repair. Done: `wow_shopping_list` (trainer, repair, ammo, pet food, materials, who sells, a budget in priority order). | M | 🟡 |
| 47 | ✅ `wow_reagents` | Reagents and consumables: how much is left and how long it lasts at the current rate of use. Done: `wow_reagents` (stocks and consumption from the history, `lowStock`). | S | 🟡 |
| 48 | ✅ `wow_quest_rewards` | An evaluation of choice rewards in quests against gear (requires item 16). Done: `wow_quest_rewards` (reward evaluation with `weights`, a choice with a justification, the `suspect` flag for repeated reward lists). | M | 🟡 |
| 49 | ✅ `wow_leftovers` | Quest items whose quests have already been turned in or abandoned, to throw away. Done: a confidence rating (`confidence`), removes nothing. | S | 🟡 |
| 50 | ✅ `wow_timeline` | A character timeline from the history: level-ups, new abilities, gear changes, quests turned in. Done in server 0.8.0: `wow_timeline` (level-ups, new abilities, gear, quests turned in, deaths, profession thresholds; filters `kind`, `days`, `limit`). An event has the time of the first dump in which it is visible. | M | ⚪ |
| 51 | ✅ `wow_compare_characters` | A comparison of two characters on the account side by side. Done in server 0.8.0: `wow_compare_characters` (level, gold, play time, professions, ilvl and gear slot by slot; arguments `a`, `b`). | S | ⚪ |
| 52 | ✅ `wow_alt_mail` | What to mail to an alt: materials for its profession lying with another character. Done in server 0.8.0: `wow_alt_mail` (strong evidence from recipes, weak and explicitly marked from the kind of material when the recipient has no saved recipes). | M | ⚪ |
| 53 | ✅ `wow_account_crafting` | Who on the account can make what and from whose materials. Done in server 0.8.0: `wow_account_crafting` (how many can be made from the crafter's materials alone and from the whole account's, who holds what, what to send). | M | ⚪ |
| 54 | ✅ `wow_buffs` | Active buffs with time remaining and a warning when standing ones are missing (e.g. Aspect). Done in server 0.8.0: `wow_buffs` (buffs, debuffs and missing standing class buffs). Time remaining only when the dump has it: see item 101. | S | ⚪ |
| 55 | ✅ `wow_flights` | Known flight points in the zones where you have quests. Done in server 0.8.0: `wow_flights` (flight points matched against quest zones; the zone from the point's name, because entries have no zone field: see item 102). | S | ⚪ |
| 56 | ✅ MCP resources | The dump as `wow://<character>/<section>` resources, attachable in a conversation without calling a tool. Done in server 0.8.0: the index `wow://index` and the template `wow://<character>/<section>` (JSON of the same section as `wow_section`); resources registered once at startup, with no change notifications. | M | ⚪ |
| 57 | ✅ Player stat weights | The player's own stat weights (an argument or a file in the server directory) used by `wow_compare_item`. No built-in builds from Classic. Done: the `weights` argument in `wow_compare_item` and `wow_quest_rewards` and the `weights.json` file (weights for a character or class) next to the goals file. | M | 🟡 |

## 5. Analysis and prompts

| # | Idea | What it is | Size | Priority |
|---|---|---|---|---|
| 58 | ✅ Prompt `przed_wylogowaniem` (`before_logout`) | What to do before leaving: an inn for rested, mail, auctions, repair, bag space. Done. | S | 🟡 |
| 59 | ✅ Prompt `pierwszy_raz` (`first_run`) | An introduction for a new character or after installation: what the assistant sees and what is worth asking about. Done. | S | 🟡 |
| 60 | ✅ Prompt `tydzien_w_grze` (`week_in_game`) | A summary of the week from the history: levels, gold, play time, the best session. Done in server 0.8.0: the prompt (gold, session journal and timeline from the last 7 days). | S | ⚪ |
| 61 | ✅ Warnings in `wow_status` | Expiring mail, ending auctions, low gear condition and full bags in one place. Done: the `alerts` field in `wow_status`. | S | 🟡 |
| 62 | ✅ When to return to town | A rule in `session_plan`: new abilities at the trainer in X levels, bags full in about Y monsters. Done: `returnToTown` in `wow_shopping_list` and in the data of the `session_plan` prompt (levels to new abilities at the trainer, hours to full bags and to the end of ammo, durability; "no data" when there is too little history). | M | 🟡 |
| 63 | ✅ Session rating | A comparison of the current session's rate with the median of previous ones. Done in server 0.8.0: `wow_session_rating` (XP, gold balance and quests per hour against the median of earlier sessions; at least three sessions are needed). | S | ⚪ |
| 64 | ✅ Make or buy | A comparison of the material cost from the auction house with the price of the finished item. Done in server 0.8.0: `wow_make_or_buy` (ingredient cost from the auction or the vendor versus the price of the finished item; a missing price is never zero, verdict `unknown`). | M | ⚪ |
| 65 | ✅ Quests along the way | Grouping quests from different zones by order of travel, with flight points. Done: `wow_quest_route` (server 0.9.0, package `game/questroute`): an order of zones by the number of quests and the current zone with flight points; this is not the shortest route (no world map), the `assumption` field says so. | M | ⚪ |

## 6. Hunter and pet

| # | Idea | What it is | Size | Priority |
|---|---|---|---|---|
| 66 | ✅ Prompt `zwierzak` (`pet_care`) | Pet state, happiness, loyalty, training points and what to improve. Done. | S | 🟡 |
| 67 | ✅ Missing ranks | From the Beast Training window and the pet's abilities: which ranks it knows, which are missing at its level. Done: `wow_pet` shows `rankReview` (ranks from the Beast Training window that the pet does not have, and ranks from trainer offers that the Hunter does not know); only from dump data. | M | 🟡 |
| 68 | ✅ Training points plan | What to spend the pet's training points on given its current abilities. Done: `trainingPlan` in `wow_pet` (free points versus the cost of trainings; an explicit ordering heuristic). | S | 🟡 |
| 69 | ✅ Food in bags | Which food from bags fits the pet's diet and for how many feedings it lasts (requires 24). Done in addon 0.22.0, server 0.22.0 (not yet run in the real client): the addon asks the client itself, `C_PetInfo.CanPetEatItem`, for every bag item that may be food (`pet.bagFood`, the argument form that answered in `checkedWith`; no diet table), and `wow_pet`, `pet_care`, `wow_shopping_list`, `wow_reagents` and `wow_bag_plan` show the food the pet eats, the food it refuses and `feedings`. Without that data in the dump the old fallback (food by item subtype, matched by the reader) stays. `/mcp probe` records the raw answers to confirm the signature. | S | 🟡 |
| 70 | ✅ Where to tame | Which animal to tame to learn a missing rank. No game API lists it and no Classic table is used: the addon learns from what the account's Hunters do in Forever. Done in addon 0.22.0, server 0.22.0 (not yet run in the real client): an account-wide store of observed beasts by creature ID (section `beasts`, at most 300) with the new pet's abilities and ranks at the moment of taming and the raw tooltip lines of beasts carrying Beast Lore; `whereToTame` in `wow_pet` and `pet_care` lists each rank seen, whether the Hunter knows it (Beast Training window) and which observed beasts have it (zone, level, coordinates, when seen), unknown ranks first, and says how to collect observations when there are none. The Beast Lore tooltip format is read tolerantly until `/mcp probe` shows the real one. | L | ⚪ |
| 71 | ✅ Ammo | How many arrows or bullets you use per hour (from the history) and how many to buy before leaving. Done: `wow_ammo`. | S | 🟡 |
| 72 | ✅ Pet comparison | A comparison of the active pet with those in the stable (requires 23). Done: the `stable` field in `wow_pet` and in the `pet_care` prompt (level and family of stabled pets versus the active one; there is no data on the abilities of stabled pets). | S | ⚪ |

## 7. Goals and session journal

| # | Idea | What it is | Size | Priority |
|---|---|---|---|---|
| 73 | ✅ Goals file | `goals.json` in the server directory (never in the game directory) with the player's goals. Done: `goals.json` next to the history directory (`--goals-file`), never in the game directory. | S | 🟡 |
| 74 | ✅ `wow_goals` | Progress toward goals (e.g. level 40 and a mount): how much XP and gold is missing, how many sessions that is at the current rate. Done: `wow_goals` (state, missing, percent, ETA from the `wow_progress` rate). | M | 🟡 |
| 75 | ✅ `wow_goal_set` | The assistant saves or changes a goal at the player's request (write only in the server's own directory). Done: `wow_goal_set` (the only tool that writes; an atomic write to the server's own file). | S | 🟡 |
| 76 | ✅ Goal templates | Ready-made types: level, gold, reputation, profession level, ability from a trainer. Done: the types `level`, `gold`, `reputation`, `profession`, `spell`, `item`. | S | 🟡 |
| 77 | ✅ Session journal | A description of every session is created automatically from the dump history: level, gold, quests, new items. Done: `wow_journal` (sessions from the history: levels, XP, gold, quests, notable items, loot, deaths, zones). | M | 🟡 |
| 78 | ✅ Journal in Markdown | An optional write of the journal as `.md` files next to the history, to read without the assistant. Done: `wow_journal` with `markdown=true` returns ready text in the `text` field; the server itself does not write `.md` files (deliberately: writes only the history, log and goals file). | S | ⚪ |
| 79 | ✅ Loot in the journal | The most valuable loot of a session in the journal (requires 18). Done: `loot` for every session in `wow_journal` and in `wow_changes`. | S | ⚪ |
| 80 | ✅ Records | The fastest level-up, the best session by gold, the longest session. Done: `records` in `wow_journal` (the longest session, the best by gold, the fastest level). | S | ⚪ |

## 8. Quality and tests

| # | Idea | What it is | Size | Priority |
|---|---|---|---|---|
| 81 | ✅ Git repository | The project has no `.git`; change history and the ability to go back. Done: a local git repository. | S | 🔴 |
| 82 | ✅ CI on GitHub | `go vet`, `go test` and `run_addon.py` for three variants on every push. | M | 🟡 |
| 83 | ✅ Contract test | Checks that every field read by the server appears in the data from `run_addon.py`. Done: `server/contract_test.go` (every section read is in some test dump, every tool works on every dump). | M | 🟡 |
| 84 | ✅ Addon lint | `luacheck` with a list of WoW globals, to catch typos and accidental global variables. | S | 🟡 |
| 85 | ✅ Mocks from the real game | The Forever mocks in `run_addon.py` fixed based on `/mcp probe` from a real client. Done (0.7.1): the `/mcp probe` result of 2026-11-15 (`testdata/real/probe_2026-11-15_addon070.json`) carried into the mocks: only `C_TradeSkillUI`, `C_AuctionHouse`, `C_StableInfo`, `C_FriendList`, `C_PartyInfo.GetLootMethod`, no PvP rank, rejected events (`FOREVER_REJECTED_EVENTS`), secrets (`issecretvalue`), an "empty" client on exiting the game; the `run_addon.py lifecycle` mode. Earlier: talents (pixel positions, descriptions). | M | 🟡 |
| 86 | ✅ Parser fuzzing | `go test -fuzz` for `savedvars.go`, so a damaged file does not take the server down. Done: `FuzzParse`, nesting limit 200. | S | ⚪ |
| 87 | ✅ Golden response tests | Saved tool responses compared on every change. Done: `server/golden_test.go`, golden files in `testdata/golden/`. | M | ⚪ |
| 88 | ✅ Prompt tests | Every prompt renders without errors on modern, legacy and forever data. Done: prompts are covered by the golden response test. | S | ⚪ |
| 89 | ✅ Splitting the addon into files | `WowMcp.lua` is 92 KB; a split into modules (sections, panel, commands) in `.toc`. Done: modules and `Sections/*` in `.toc`. | M | ⚪ |
| 103 | ✅ Localized game clients | The server works on non-English clients (deDE, frFR, ruRU...). Done in addon 0.9.0 / server 0.11.0: the addon saves `character.locale`, item `classID` / `subclassID` / `bindType` / `statTokens`, `isBound`, `isQuestItem`, `questID`, profession `skillID` and pet `petAction`; the server classifies items in `game/itemkind`, matches professions by skill line ID and stats by token, and marks the remaining English-only heuristics with `localeNote`. Tested on mocks only: the `dede` harness mode, the `WowMcp_dede` golden scenario and parity tests. | M | 🟡 |
| 104 | Verify a localized client | Run addon 0.9.0 in a non-English client (e.g. deDE) and compare with the mock: item IDs, `statTokens`, `skillID` of the old Skills window, loot and XP parsing with the client's format strings. | S | 🟡 |
| 105 | ✅ Class buffs by spell ID | `wow_buffs` matched missing class buffs by English spell and aura names. Done in addon 0.22.0, server 0.22.0 (not yet run in the real client): the addon saves the class's buff groups as spell IDs with the names the client returns in its own language (`classBuffGroups`; all ranks of a spell share a name), and the server matches known spells and active auras by those names, so `missing` works in any game language. The IDs come from the original game: on an English client the server cross-checks each returned name against its English rule, drops a spell that does not fit and reports it (`groupIdMismatches` in `wow_buffs`, check `class buff spell IDs` in `wow_doctor`). Dumps without the section behave as before (`localeNote`, `selfBuffsNotActive`). | S | 🟡 |
| 106 | ✅ Dungeons and flight points without English names | `wow_dungeons` uses a table of English classic dungeon names when the client gives no list, and `wow_flights` matches points without `zone` by English zone abbreviations. Needs map or instance IDs from the client. Done in addon 0.10.0 and server 0.12.0 (map and instance IDs); the classic fallback dungeon table stays English. | M | ⚪ |
| 107 | ✅ Remaining English text | Weapon speed in `wow_compare_item` is read from English tooltip lines (no stat token; a candidate for `C_Item.GetItemInfo`-based data or a speed field from the addon); on classic clients the Beast Training window is found by its English name in `recipes` (Forever shows it as a trainer window without an NPC, which works on any client); pet food and bandages fall back to the English item name; the `type` filter of `wow_bank` / `wow_vendor` takes English class names (subtype words match only the client's text); goals compare the faction, profession, spell and item names the player types with the client's names, so on a localized client they must be given in the client language (standings use English names with a numeric fallback). Done in addon 0.10.0 for weapon speed and DPS (client format strings); some English-only bits remain. | M | ⚪ |

## 9. Performance and reliability

| # | Idea | What it is | Size | Priority |
|---|---|---|---|---|
| 90 | Checked, deliberately not doing: parse cache | The server does not parse the file again when its modification date has not changed (check whether it already works this way). Checked: parsing takes about 0.4 ms, and domain code appends fields to the dump maps in place, so a shared cache would leak fields between calls. | S | 🟡 |
| 91 | ✅ File being written | When the game is writing the file at that moment, the server waits a moment and tries again instead of returning an error. Done: up to 4 attempts in about 600 ms. | S | 🟡 |
| 92 | ✅ Dump time | The addon measures how long the dump of each section takes and warns in `/mcp status` when the whole takes too long. Done: `timings` in the dump, a warning in `/mcp status` from 200 ms. | S | 🟡 |
| 93 | ✅ Server log | The `--log` flag with errors written to a file in the server directory. Done: `--log`, never in the game directory. | S | 🟡 |
| 94 | ✅ Lighter history | Compression of old history entries (gzip) and a limit in MB alongside the limit on the number of entries. Done: `--history-max-mb` (default 200) and `--history-compress-days` (default 7) in server 0.9.0; reading supports `.json` and `.json.gz`. | S | ⚪ |
| 118 | ✅ Loot and XP survive double saves | The real run of 2026-11-16 lost a whole session of loot and XP: `/mcp reload` saved it, the logout 10 s later saved an empty period over it, and the server reads the file only when a tool is called. The addon never assumes the server read a dump: closed periods with content are carried in `loot.periods` / `xpGains.periods` (at most 20, at most 7 days old, empty ones not stored), and the server merges those that started after the previous history snapshot, without double counting (`periodsMerged`). Done in addon 0.12.0 and server 0.13.0. | M | 🔴 |
| 119 | ✅ Flight points with a continent as zone | The real dump had `zone = "Eastern Kingdoms"` on every flight point (flight map captured by addon 0.7.0), so `wow_flights` matched no zone. A continent name counts as no zone, a zone that matches no quest zone falls back to the name, and the addon writes a continent-only position as `continent`. Done in addon 0.12.0 and server 0.13.0. | S | 🔴 |
| 120 | ✅ `wow_section` limit one level down | `limit` without `path` returned all 77 dungeon activities, because the section's only key is a list; lists directly inside the object are now cut too, with `limited.lists`. Done in server 0.13.0. | S | 🟡 |

## 10. Documentation

| # | Idea | What it is | Size | Priority |
|---|---|---|---|---|
| 95 | ✅ `CHANGELOG.md` | What changed in each addon and server version. Done. | S | 🔴 |
| 96 | ✅ How to ask your assistant | Example questions for every tool and prompt. Done: `docs/HOW_TO_ASK.md`. | S | 🟡 |
| 97 | ✅ ToS compliance | A short document: what the addon and server do, what they will never do and why. Done: `docs/TOS_COMPLIANCE.md`. | S | 🟡 |
| 98 | ✅ Dump format description | A schema of the sections and fields of `WowMcp.lua` with the version in which they appeared. | M | ⚪ |
| 99 | Screenshots | The in-game panel and an example conversation in the README. | S | ⚪ |
| 100 | ✅ Troubleshooting | An expanded "When something does not work" section based on errors from the real client. | S | 🟡 |

---

## 11. International release

| # | Idea | What it is | Size | Priority |
|---|---|---|---|---|
| 108 | ✅ Product in English | Addon messages, tool texts, prompts (renamed), documentation and comments in English. Done in addon 0.8.0, done in server 0.10.0. | L | 🔴 |
| 109 | ✅ Release archives for Windows, macOS and Linux | `make dist` builds per-platform archives; `install.sh` for macOS and Linux; release workflow. Done in server 0.11.0. | M | 🔴 |
| 110 | ✅ License | GPL-3.0-or-later with an additional term (section 7(b)) that the author attribution must be kept: `LICENSE`, `NOTICE`. Done in server 0.11.1. | S | 🔴 |
| 111 | ✅ MCP tool annotations | `readOnlyHint` / `idempotentHint` / titles on every tool (only `wow_goal_set` writes), so clients can auto-approve read-only tools. Done in server 0.12.0. | S | 🟡 |
| 112 | ✅ Response size control | A common `limit` / `fields` convention and a size cap for large answers (`wow_section probe` is 60 kB), so answers fit small context windows. Done in server 0.12.0: `--max-response-kb` envelope, `wow_section` `path` / `limit`. | M | 🟡 |
| 113 | ✅ `wow_doctor` | One diagnostic tool: file found, addon and server versions, age, missing windows, dump errors, unloaded dumps, locale notes, what to do next. Done in server 0.12.0. | S | 🟡 |
| 114 | ✅ Several accounts and realms | Choose the account folder when `WTF/Account` has more than one; characters with the same name on different realms. Done in server 0.12.0: `--account`, realm-aware character lookup. | M | ⚪ |
| 115 | ✅ Publishing checklist | What the owner has to do to publish: repository, first tag, MCP registry entry, CurseForge / Wago addon page. Done: `docs/PUBLISHING.md` (the steps themselves are for the owner). | S | ⚪ |
| 116 | ✅ Independent code review | A critical review of the whole codebase before publishing: safety guarantees, correctness, architecture, honesty of outputs, tests, release hygiene. Done: `docs/CODE_REVIEW_2026-11-16.md`; the fixes shipped in addon 0.10.1 / server 0.12.1. | M | 🔴 |
| 117 | ✅ Review follow-ups | Recommendations from the review that were not done: `staticcheck` in CI, an `install.ps1` test on Windows (junction guard, `-Client` check), a decision about `testdata/real/` (character, realm, macro) before the repository goes public, a guard against the logout check when the player disabled three of its four sections. Done: the logout guard no longer depends on `/mcp off` (addon 0.11.0: a disabled sign section is read for the check only) and prompts are capped by `--max-response-kb` (server 0.12.2). Done after 0.20.0: `staticcheck` runs in CI, `scripts/test-install.ps1` runs the installer on a Windows runner under Windows PowerShell 5.1 and PowerShell 7 (`-Client` check, fresh install with `-Build`, mirror and backup, `-Rollback`, junction guard), and `testdata/real/` stays: the repository went public with it (a character and realm name, no guild, mail or friends). | S | 🟡 |
| 121 | ✅ Compare auction house gear with the equipped gear | Price entries carry item details from the client cache only (no request, no tooltip); `wow_auctions` gives `vsEquipped`, `canAfford`, `levelOk` and the `type`, `affordable`, `usable`, `equippable` filters like `wow_vendor`. Found when the player asked which bow to buy. Done in addon 0.13.0, server 0.14.0 (not yet run in the real client). | M | 🟡 |
| 122 | ✅ Quest locations and guide steps | Where a quest starts and ends, from the quest database installed in the game (QuestieDB) instead of memory or quest text, and the next steps of the player's Guidelime guide (TUGs). Done in addon 0.14.0, server 0.15.0: sections `questDb`, `guide`, `questLookups`, `/mcp quest`, tools `wow_quest_lookup`, `wow_guide`. | M | 🔴 |
| 123 | ✅ Legacy perks and challenges | The Forever Legacy system (Legacy Points, the Professions / Adventure / Resourcefulness perk trees, Legacy challenges) so advice on Legacy Points needs no screenshots. The client API is undocumented, so the addon reads every non-class `C_Traits` config, the Legacy achievement category and records the client's Legacy API names (`discovery`). Done in addon 0.15.0, server 0.16.0: section `legacy`, tool `wow_legacy` (not yet run in the real client). | M | 🔴 |
| 126 | ✅ Item and spell lookup | The real in-game description of any item or spell instead of a guess from another game version. The server cannot ask the game, so the player types `/mcp item <id, link or name>` or `/mcp spell <id, link or name>`: the addon reads the client (name, tooltip text, stats, rank, cast time, cost, range), handles items the client has not cached yet, and keeps the last 30 answers. `wow_lookup` searches them and everything else the dump already knows by name or ID, says where each hit came from, and tells the assistant what to ask the player when nothing is found. Done in addon 0.21.0, server 0.21.0 (not yet run in the real client): section `lookups`, tool `wow_lookup`. | M | 🟡 |
| 127 | ✅ Guild overview and group candidates | "Who in my guild is online, which classes and levels, and with whom could I group?" Done in `wow_social`: roster `summary`, `groupCandidates` by tank-, healer- or damage-capable class (`levelRange`), guild `note` and `offlineDays` in the `guild` section, `classToken` in `guild` and `friends`. Done in the addon and the server (not yet run in the real client). | M | ⚪ |
| 128 | ✅ PvP history | Honor and honorable kills in the long history (`history.db` schema 4, nullable columns, older databases migrate in place), shown as `honor`, `honorableKills`, `honorLevel` in `wow_changes` and `honorGained`, `honorableKills` per session and overall in `wow_progress`, only when the dump has `pvp`. Done in the server (not yet run in the real client). | M | ⚪ |
| 129 | ✅ Zone quest progress | How many of a zone's quests the character has done, which are in the log and which are left (with quest givers), and which quest chains stopped half way, from the quest database installed in the game (QuestieDB) and the completed quest list; quests for another faction, race or class are left out. Done in addon 0.21.0, server 0.21.0 (not yet run in the real client): section `zoneQuests`, tool `wow_zone_progress`. | M | 🟡 |
| 130 | ✅ Dungeon loot upgrades | Which items a dungeon's bosses drop that would be upgrades for the character, read from the game's Encounter Journal when the player types `/mcp loot [dungeon]` (never on a dump; the journal's selection and filters are put back) and compared with the equipped gear by the `wow_compare_item` rule, after a class armor and weapon check. The server has no drop tables. Done in addon 0.21.0, server 0.21.0 (not yet run in the real client): section `dungeonLoot`, tool `wow_dungeon_loot`, `/mcp probe` lists `EJ_*` and the `Blizzard_EncounterJournal` state. Real client (WoW: Forever, build 71037): the `EJ_*` functions exist but the client has no Encounter Journal and refuses to load its module (`WRONG_GAME_TYPE`), so the tool only reports that there; the read itself is untested and needs a client with a journal or another drop source. | M | 🟡 |
| 131 | ✅ Share an anonymized dump | Only a Hunter and a Warlock were checked in the real game. `wow_share_dump` and `wow-mcp --share-dump` write an anonymized copy of a dump (names replaced, personal text removed, unknown sections left out) that the player reads and attaches to the "Share a dump from your class" issue form; nothing is uploaded. Accepted dumps go to `server/testdata/shared/` and every tool is run on them. | M | 🟠 |

---

## Dependencies

- 48 required 16, 72 required 23, 79 required 18 (all done); 69 requires 24.
- 74–76 require 73. 77 uses the same history as `wow_progress`.
- 9, 10, 11, 12 extend the installer from item 2.
- 85 and 100 are best done after the results from items 3–6.

## Rejected (they break the project's principles)

- Automatic saving after events (`/mcp auto on`, auto `/reload` after a trainer window, a button after leveling up). Rejected earlier by the owner.
- Automatic selling of junk, repair, accepting and turning in quests, buying from trainers: actions in the game without a player click.
- Automatic searching or listing at the auction house, scanning the whole auction house.
- Queries about other players without a click (automatic Inspect, `/who`).
- Reading the game process memory, network traffic or screen pixels; overlays on the game window.
- Fetching data from external services (e.g. item databases) through the server: the server reads only the dump file.
- Built-in "popular builds" of talents from Classic: the Forever trees are different.
