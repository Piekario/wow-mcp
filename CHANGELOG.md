# Changelog

The format is modeled on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). The WowMcp addon and the wow-mcp server have
separate version numbers. In a real Forever 1.60.1 client the following were run: addon 0.5.0 with server 0.6.0 (2026-11-15, no section
errors, dump 18 ms), addon 0.7.0 with server 0.9.0 (2026-11-15, no section errors, dump 29 ms) and addon 0.11.0 with server 0.12.2
(2026-11-16, no section errors, dump 26.1 ms; `testdata/real/snapshot_2026-11-16_addon0110.json`). The changes of the versions in
between are part of 0.11.0 / 0.12.2 and were run with them. Addon 0.14.0 was run on 2026-11-22 (client build 71037, no section errors, dump 25.7 ms: `questDb` for 22 log quests, `guide` reading TUGs, `/mcp quest The Glowing Shard`). **Server 0.15.0 and the 0.12.0 / 0.13.0 addon changes (carried periods, flight zones, auction gear details) have not yet been checked in a real client**: their fixes are
based on the real dumps (`testdata/real/`), API mocks matching the `/mcp probe` result and golden response tests.

## 0.22.0 (unreleased)

Pet food the client checks, where to tame missing ranks, and class buffs by spell ID, for the addon and the server
(roadmap 69, 70 and 105). The server expects addon 0.22.0. Run in the real client on 2026-11-24 with a Warlock (build 71037, no section errors, dump 43.5 ms), which confirmed:
`C_PetInfo.CanPetEatItem(itemID)` returns one boolean and raises an error for an item link (`probe.petFood`); every new
event is accepted (`PLAYER_TARGET_CHANGED`, `UPDATE_MOUSEOVER_UNIT`, `UNIT_AURA`, `UNIT_PET`, `PET_BAR_UPDATE`,
`SPELLS_CHANGED`, `UNIT_SPELLCAST_START`, `UNIT_SPELLCAST_CHANNEL_START`); `classBuffGroups` gave Demon Skin (687) and
Demon Armor (706) with the right names and left out Fel Armor (28176), which this client does not know. Not yet run
with a Hunter, so the points below about taming, Beast Lore and the pet's food list are still assumptions:

- Tame Beast is spell ID 1515 and fires `UNIT_SPELLCAST_START` or `UNIT_SPELLCAST_CHANNEL_START` with `(unit, castGUID,
  spellID)`; the tamed pet arrives with `UNIT_PET`, `PET_BAR_UPDATE` or `SPELLS_CHANGED` within 60 s; the pet's GUID has
  the form `Pet-0-…-<npcID>-…` (otherwise the families are compared); creature GUIDs have the form
  `Creature-0-…-<npcID>-…`.
- Beast Lore is spell ID 1462 and leaves an aura on the beast that `C_UnitAuras.GetUnitAuraBySpellID` (or, without it,
  `C_UnitAuras.GetAuraDataByIndex` / `UnitAura`) finds; the beast's abilities appear in the unit tooltip that
  `C_TooltipInfo.GetUnit` returns. The tooltip format is unknown, so the lines are saved raw.
- Class buff spell IDs (`ClassBuffs.lua`) are the IDs of the original game and are not verified for Forever;
  `C_Spell.GetSpellName` answers with the spell's name in the client's language, and all ranks of a spell share that
  name. On English clients the server cross-checks every returned name against its English rule and reports an ID that
  does not fit; on other clients the IDs cannot be cross-checked.
- Events `PLAYER_TARGET_CHANGED`, `UPDATE_MOUSEOVER_UNIT`, `UNIT_AURA`, `UNIT_PET`, `PET_BAR_UPDATE`, `SPELLS_CHANGED`
  and the two spellcast events are accepted by the client (`probe.events` shows it).

- **Fixed**
  - A recipe window for which the client gives no profession name was saved under an empty name, and `wow_recipes`
    showed a profession called `""` (seen in the real client on 2026-11-23: one recipe, Camp Chair, `professionName`
    empty, skill 0/0). The addon now saves it as `Unnamed profession`, keeps the client's own fields in `windowInfo`
    so the missing name can be traced, and drops the old entry; the server shows dumps of older addons the same way.
  - The `friends` section was never saved in Forever build 71037: opening the friends list fires no `FRIENDLIST_UPDATE`
    there (real client, 2026-11-24). Every dump now reads the list the client already holds, still without asking the
    server for it; an empty list read this way is not saved, because the client reports no friends before it has the
    list. Battle.net friends who are in the game join the list as their character (`battleNet = true`); the account
    name and BattleTag are not read. Assumes `BNGetNumFriends` and `C_BattleNet.GetFriendAccountInfo(index)` with
    `gameAccountInfo` (not yet confirmed in the real client; `/mcp probe` now lists the `BNGet*` globals).

- **Added**
  - Sharing a dump with the project, so classes, factions, clients and languages nobody has tested can become test data.
    The new tool `wow_share_dump` and the command `wow-mcp --share-dump` (`--share-character`, `--share-dir`) write an
    anonymized copy of a character's latest dump into the server's own directory, by default `shared` next to the
    history. Character, guild, friend, group and inspected player names become aliases (`Player`, `Player 2`,
    `Guild 1`; the same player keeps one alias everywhere), mail senders become `Sender 1`, the realm name and number
    and player GUIDs are dropped, and the note, macro bodies, mail subjects, guild and friend notes are removed. The
    collected names are then searched for in every other text (a quest text greeting the character, `<Made by ...>` in
    a tooltip), and the file is not written if one is still found. A section without privacy rules is left out.
    Class, race, level, client version, language, all game data and the `/mcp probe` result stay.
  - In game, like QuestieTrace: `/mcp share` opens a window with the address of the issue form, a short client report
    as one line of text to copy (Ctrl+C), a few counters and the buttons "I reported this" and "Close". The report is built from a fixed list of fields (addon and client version, build, language, class,
    race, level, section names and sizes, addon errors, rejected events, API namespace sizes after `/mcp probe`) and
    holds no names, items or gold. A chat line with a link to the window is printed at most once a month and three
    times in all, only for a class, client build and language the account has not marked with "I reported this", and never
    before the character has a dump. `/mcp share off` or the new "Sharing reminder" switch on the options page turns it off. The addon sends nothing; a client without timers gets no reminder.
  - Chat links of the addon share one handler (`ChatLinks.lua`); the TomTom waypoint links work as before.
  - Nothing is uploaded: the server still opens no network connection (now checked by a test). The player reads the
    file and attaches it to the new issue form "Share a dump from your class", which also takes the client report. `wow_status` shows `shareDir`.
  - Accepted dumps live in `server/testdata/shared/`; `TestSharedDumps` requires each to be exactly what the current
    rules would write and calls every tool on it. The first two are the project's own real Hunter dumps.
  - [docs/SHARING_DUMPS.md](docs/SHARING_DUMPS.md): what is removed, what stays, and what happens to an accepted dump.
  - Addon: section `classBuffGroups` (`ClassBuffs.lua`, `Sections/ClassBuffGroups.lua`): the player's class buff groups
    (Aspects, Blessings, mage and warlock armors, shamans' shields, Arcane Intellect, Fortitude, Mark of the Wild, Battle
    Shout) as `{label, spells = {{id, name}}}`, one ID per spell, with the name the client gives it in its language. An
    ID the client does not know is left out; a class without groups has no section.
  - Server: `wow_buffs` matches the groups by those names, so `missing` works on non-English clients and the `localeNote`
    and the `selfBuffsNotActive` fallback appear only for dumps without the section. On an English client a returned name
    that the English rule of the group (`rules()`) does not accept means the ID is wrong for this game version: the spell
    is dropped from the group and listed in `groupIdMismatches`, and `wow_doctor` warns in the new check `class buff
    spell IDs` (shown only when the dump has the section). The Hunter's pet warnings (Call Pet, Revive Pet) still match
    English names.
  - Addon: `pet.bagFood` (`PetFood.lua`): for every distinct bag item that may be pet food (Food & Drink, and Trade Goods
    because raw meat and fish are trade goods) the addon asks the client `C_PetInfo.CanPetEatItem` and saves
    `{checkedWith, items = {id, name, count, canEat}}`. Nothing is saved when the client has no such function or never
    answers with a boolean. The addon only asks; it never feeds the pet.
  - Addon: section `beasts` (Hunters only; `Beasts.lua`, `BeastTaming.lua`, `BeastLore.lua`, `Sections/Beasts.lua`): an
    account-wide store of observed beasts keyed by creature ID (`WowMcpDB.beasts`, at most 300, the least recently seen
    dropped), carried in every Hunter's dump. When the player tames a beast, the new pet's spellbook abilities with ranks
    are saved at that moment (`source = tamed`); when the target or mouseover unit carries Beast Lore, its tooltip lines
    are saved raw (`source = beastLore`), with name, level, family, zone, map ID and the player's position. `/mcp off
    beasts` stops the recording.
  - `/mcp probe` also saves `petFood` (the raw answers of `CanPetEatItem` for the first 3 bag items, with each argument
    form) and `beastLore` (the target's tooltip lines when it carries Beast Lore), so one run confirms the signature and
    the tooltip format.
  - Server: `wow_pet` shows `bagFood` (food the pet can eat, food it refuses, `feedings`) and `whereToTame`: each ability
    rank seen on observed beasts, whether the Hunter knows it (from the Beast Training window; unknown when it was never
    opened) and which beasts have it (name, level, zone, coordinates, source, when seen), ranks the Hunter does not know
    first. Beast Lore lines are read tolerantly: `Bite (Rank 3)`, `Bite Rank 3` (with the rank word the client uses
    elsewhere in the dump, so any language works) or `Bite 3` for an ability name the dump already knows. The answer says
    the data comes only from beasts this account tamed or examined, and how to collect them when there are none.
- **Changed**
  - CI runs `staticcheck` on the server and tests `install.ps1` on a Windows runner, under Windows PowerShell 5.1 and
    PowerShell 7 (`scripts/test-install.ps1`: a `-Client` value that leaves the game folder is refused, a fresh install
    with `-Build`, a second install that backs up and removes stale files, `-Rollback`, and an addon folder that is a
    junction is refused with the linked folder left alone). This closes roadmap item 117.
  - `pet_care`, `wow_shopping_list`, `wow_reagents` and `wow_bag_plan` use the client's verdict on the bag food when the
    dump has it: feedings are counted, refused food is named, raw meat the pet eats is kept instead of banked. Older dumps
    keep the food by item subtype with the note that matching it to the diet is left to the reader.
  - The player's map position is read by one helper (`Maps.playerPosition`), shared by `character` and `beasts`.

## 0.21.0 (unreleased)

Seven new tools and two extended ones, for the addon and the server. The server expects addon 0.21.0. **Apart from the first run described below, none of this has been run in the
real client**: everything below was checked only on the addon's API mocks, unit tests and golden responses, and each part
names the client API it assumes.

Found in the first real-client run (2026-11-24, build 71037): addon 0.21.0 ran with zero section errors and the dump took 43.5 ms
(achievements 8.3 ms). The achievement and statistics API exist (111 achievements, 63 statistics); `zoneQuests` works with QuestieDB and
`/mcp item` works. Titles: 0 known of 111. `C_MountJournal` and `GetCompanionInfo` exist but their lists are empty;
`C_TitleManager` is absent. The `EJ_*` globals exist, but no `dungeonLoot` section was produced yet. The run showed three problems,
fixed here:

- **Fixed**
  - `wow_social` group candidates listed the character itself when the guild roster wrote its name with a realm after a space
    ("Name Realm") instead of a dash. Both forms are now recognised.
  - Legacy challenges: the achievements that grant Legacy Points are in ordinary categories (Classes > Warlock, Tradeskills >
    Alchemy, Dungeons, ...), no category title contains "Legacy", and all achievements are worth 0 points. They are recognised by
    their reward text "Earn 1 Legacy Point." The addon now matches an achievement as a challenge when its category title mentions
    Legacy or its reward text mentions "Legacy Point" (`challenges.matchedBy` says which rule matched), reading from the same pass
    as the `achievements` section instead of reading every achievement twice; `categoryID`, `completedOn` and the criteria of
    incomplete ones are saved too. `wow_legacy` also derives the challenges from the `achievements` section by the reward text
    (English client text) when a dump's own list is empty, so dumps from before this fix work; `challenges.source` and
    `matchedBy` say how they were found, `completedList` has the dates and open ones the `missing` criteria.
  - `wow_social` was 59.5 KB for a guild of 937 members and was cut at the response limit. The guild member list is now capped:
    `limit` (default 40, the members closest to the character's level first, then by name; -1 = every matching member) and
    `query` (name, class, zone, rank or note); `memberList` states `shown`, `total`, `notShown` and how to get the rest. A
    synthetic guild of 300 members answers in 14.8 KB with the default arguments (a test keeps it under 20 KB).
  - Dungeon loot: the Encounter Journal lists one tier at a time, so only the current tier's dungeons could be found. The addon
    now lists the instances of every tier (up to 12), selects the instance's tier before reading it and puts the tier, instance
    and boss selection back. Every `/mcp loot` now leaves `dungeonLoot.lastAttempt` (outcome `read`, `noMatch`, `readFailed` or
    `unavailable`, how many instances and tiers the journal offered, the first 40 names, whether the player was in an instance,
    the error), and the section exists even when nothing was read. `wow_dungeon_loot` shows the last attempt and `journalOffers`,
    the dungeon names to type. When the journal offers nothing because `Blizzard_EncounterJournal` is not loaded,
    `/mcp loot` loads that module itself (`C_AddOns.LoadAddOn`, the game's own UI module, the same thing opening the journal
    window does), says so in chat and reads again. **WoW: Forever has no Encounter Journal**: the real client (build 71037)
    reports the module as loadable but refuses to load it (`WRONG_GAME_TYPE`), has no journal window, and the `EJ_*` functions
    list nothing. There `/mcp loot` says so in chat and records `outcome = "unavailable"` with `loadRefused`, and
    `wow_dungeon_loot` passes that on instead of asking the player to open a window. The feature stays for clients that
    have a journal.
  - Section order: `legacy` now follows `achievements` in the dump.

Item and spell lookup, for the addon and the server. Not yet run in the real client: it is
covered by the API mocks of the addon test and the golden tests only.

- **Added**
  - `/mcp item <ID, item link or exact name>` and `/mcp spell <ID, spell link or name>`: the addon reads the item or spell
    from the client (the same details and tooltip lines as bag items; for spells the name, rank, description, cast time,
    cost, range and tooltip) and keeps the last 30 answers in the new dump section `lookups`. An item or spell the client
    has not cached is requested from the client cache and saved when the data arrives; chat says so.
  - Tool `wow_lookup` (`query`, `kind`, `character`): returns the saved lookups and also what the dump already holds under
    that name or ID (bags, bank, equipment, vendor and trainer offers, quest rewards, auctions, mail, loot, recipes, known
    spells), with the section of each hit. When nothing is found it says which commands the player should type
    (`/mcp item <query>` or `/mcp spell <query>`, then `/mcp reload`) and the assistant is told not to answer from memory.

Guild overview and PvP history, for the addon and the server. **Not run in the real client**: checked only with the API
mocks, unit tests and golden responses, so the new guild fields (`note`, `classToken`, `daysOffline`) and
`GetGuildRosterLastOnline` still have to be confirmed in the game.

- **Added**
  - Guild overview in `wow_social`: a `summary` of the roster (members listed and online, counts by class, level bands
    of ten, average online level) and `groupCandidates`: online guild members and friends within `levelRange` levels
    (default 5) of the character, sorted into `tankCapable`, `healerCapable` and `damageCapable` by what their class can
    do (one class-to-role table in the `group` package), with level, class, zone, guild rank and note. A person on both
    lists appears once. A character without a guild still gets candidates from the friends list. The guild entries also
    show `note` and, for offline members, `offlineDays`.
  - Addon: the `guild` section saves each member's `note` (up to 60 characters; the officer note is never read),
    `classToken` and, for offline members, `offlineDays` (`GetGuildRosterLastOnline`); the `friends` section saves
    `classToken`, found from the localized class name, so roles work on non-English clients.
  - PvP in the long history: `history.db` now keeps honor and honorable kills for every dump that has a `pvp` section
    (schema 4: two nullable columns on `points`; older databases are migrated in place and their rows stay unknown, never
    zero). `wow_changes` shows `honor`, `honorableKills` and `honorLevel` between two dumps, and `wow_progress` shows
    `honorGained` and `honorableKills` for the session and the whole history. Both appear only when the dumps have a
    `pvp` section. The honor counter restarts at an honor level-up, so a stretch with a level-up undercounts honor and says
    so through `honorLevel`.

Zone quest progress, for the addon and the server. **Not yet run in the real client**: the addon side was checked only on the
API mocks (`addon/tests/run_addon.py`, with an invented QuestieDB), the server side with unit and golden tests. The QuestieDB
calls it relies on (`Quest.GetAllIds`, the `zoneOrSort`, `preQuestSingle`, `preQuestGroup`, `exclusiveTo`,
`breadcrumbForQuestId`, `specialFlags` and `requiredMaxLevel` fields, `Enum.raceMaskById`, `Enum.classKeys`,
`Enum.factionRaceMasks`) are taken from that addon's public API documentation, not from a run in the game.

- **Added**
  - Section `zoneQuests` (`Sections/ZoneQuests.lua`, foundation file `QuestZones.lua`): every quest the installed quest
    database files under the current zone and under the zones of the log quests (up to 3 zones, 150 quests each), in a
    compact form: name, level, required and maximum level, race and class masks, prerequisite and next quests, the quest
    it closes or leads to, repeatable, and the first quest giver with its coordinates in that zone. It also has the
    character's race and class bits and both factions' race masks. The database has no zone index, so the first dump of
    a game session walks its quest IDs once (one field per quest) and keeps the index; each zone's list is built once
    per session. Absent without QuestieDB; `noZoneIndex` when the database cannot list its quests. It only reads.
  - Tool `wow_zone_progress` (domain package `game/zoneprogress`): for a zone (`zone`, by default the character's current
    zone) how many of its quests are done (`completedQuests`), in the log and left; the quests left with level, required
    level, state (available, needs a higher level, needs an earlier quest) and quest giver; and the unfinished quest
    chains (the next quest of a chain whose earlier step is done). Quests the database marks for another faction, race or
    class are left out of the counts and counted in `notForCharacter`; repeatable quests and quests closed by an
    alternative or above their maximum level are counted apart. Without the section the answer says what to install.
- **Changed**
  - `QuestDb.lua` exposes its database accessor and field read (`QuestDb.lib`, `QuestDb.field`) for `QuestZones.lua`.

Achievements, statistics, titles and collections, for the addon and the server. **None of this has been run in the real client.**
Which of these APIs Forever has is not known yet: every name below is an assumption taken from the documented modern API, to be
confirmed with `/mcp probe`. Each section is absent when the client supplies nothing, and the tools say so.

- **Added**
  - Addon: an `achievements` section: achievement points (`GetTotalAchievementPoints`), completed and total
    (`GetNumCompletedAchievements`), categories with their counts (`GetCategoryList`, `GetCategoryInfo`,
    `GetCategoryNumAchievements`), and up to 600 achievements with name, points, description, reward and the completion
    date (`GetAchievementInfo`; the year is read as years since 2000, stored as `completedOn` = `YYYY-MM-DD`). Incomplete
    ones carry their criteria (`GetAchievementNumCriteria`, `GetAchievementCriteriaInfo`: up to 8, the missing ones first;
    25 read per achievement, 3000 per dump). The category layout and completed achievements are read once per session, so
    later dumps re-read only the incomplete ones. The Legacy challenges now read their categories through the same helper
    (`AchievementCategories.lua`); their output is unchanged.
  - Addon: a `statistics` section (`GetStatisticsCategoryList`, `GetStatistic`): the game's counters by category with the
    game's own value text, coin icons turned into `5g 20s 3c`, `--` values left out; up to 400.
  - Addon: a `titles` section (`GetNumTitles`, `IsTitleKnown`, `GetTitleName`, `GetCurrentTitle`) and a `collections`
    section: mounts from `C_MountJournal` or the companion API (`GetNumCompanions` / `GetCompanionInfo` with `"MOUNT"`),
    companion pets from the companion API (`"CRITTER"`) or `C_PetJournal`; up to 300 of each. Nothing changes the title,
    summons, dismisses or tracks anything.
  - `/mcp probe` also lists `C_MountJournal`, `C_PetJournal`, `C_TitleManager` and the global title, companion and
    comparison-statistic functions, so the next probe from the game answers which of the above exist.
  - Server: `wow_achievements` (`view` = `summary`, `timeline`, `recent`, `closest` or `all`; `query`, `category`, `limit`;
    package `game/achievements`): totals, progress per category, the most recent ones, completion dates in order with the
    earliest and latest, and the incomplete achievements ranked by criteria progress with the missing criteria.
  - Server: `wow_statistics` (`query`, `category`) and `wow_collections` (`kind` = `titles`, `mounts`, `companions` or
    `all`; `query`; package `game/collections`). `wow_search` searches the four new sections and `wow_section` lists them.
  - Tests: the modern, Forever and deDE mocks have these APIs (journals in the modern mock, the companion API in Forever);
    the legacy client has none and proves the sections stay absent. In the modern mock the Legacy challenges now report an
    empty Legacy category list, since that client has the achievement API.

Dungeon loot upgrades. **Not run in the real client yet**: the addon side was tested only against API mocks in
`addon/tests/run_addon.py`. The real `/mcp probe` of 2026-11-15 lists `C_EncounterJournal` (including `GetLootInfoByIndex`
and `GetInstanceForGameMap`), but nobody has checked whether Forever has the global `EJ_*` functions, so these names are
assumptions taken from mainline clients: `EJ_GetInstanceByIndex`, `EJ_SelectInstance`, `EJ_GetEncounterInfoByIndex`,
`EJ_SelectEncounter`, `EJ_GetNumLoot`, `EJ_GetLootFilter` / `EJ_SetLootFilter` / `EJ_ResetLootFilter`, the
`EncounterJournal.instanceID` / `encounterID` frame fields, `C_EncounterJournal.GetSlotFilter` / `SetSlotFilter` /
`ResetSlotFilter` (probed), the return order of `C_EncounterJournal.GetLootInfoByIndex` and the `EJ_LOOT_DATA_RECIEVED`
event. When they are missing, `/mcp loot` says so and records only the reason.

- **Added**
  - `/mcp loot [dungeon]` reads from the Encounter Journal what the bosses of a dungeon drop, only when the player types
    it and never on a dump. Without a name it reads the dungeon the character is in, otherwise the dungeons for the
    character's level from the saved group finder ranges (at most 3 per command). The read selects instances and
    bosses and resets the class and slot loot filters, then puts the journal window's selection and filters back; chat
    says what was read. Items the client has not loaded yet are
    marked `pending` and read again up to 5 times as their data arrives; chat says how many are still loading.
  - Dump section `dungeonLoot` (carried over between dumps): up to 8 dungeons with name, journal and map IDs, level range
    when known, boss names and up to 40 items each (ID, name, quality, boss, journal slot and armor text, item level,
    required level, stats and tooltip from the same item helpers as bags and vendors). See `docs/DUMP_FORMAT.md`.
  - `/mcp probe` also lists the global `EJ_*` functions, `IsInInstance`, `GetInstanceInfo`, `IsAddOnLoaded`,
    `GetAddOnInfo`, the `C_AddOns` namespace and whether `Blizzard_EncounterJournal` is loaded, loadable and has its frame.
  - Tool `wow_dungeon_loot` (`character`, `dungeon`, `slot`, `upgradesOnly`, default true): the saved dungeon items that
    would be upgrades, grouped by dungeon and boss, with the stat differences against the equipped item from the same
    rule as `wow_compare_item` and `wow_quest_rewards` (`items.Buyer.Evaluate`; a `weights.json` file adds stat weights).
    Items the class cannot use are left out and counted under `skipped`, from a classic armor and weapon proficiency
    table (`itemkind`, mail and plate from level 40); Forever may differ, and the answer says so. Without the section the
    answer says what to type in the game and suggests `/mcp probe`; the server has no drop tables and never guesses them.

## 0.20.0 (2026-11-24)

A long history and less `/mcp reload`, for the addon and the server. The server expects addon 0.20.0. This release also carries
0.19.2 below, which was merged but never tagged. Run in the real client on 2026-11-24 (build 71037, no section errors):
`/mcp probe` accepted the logout events and all six `eventLog` events, a plain logout left a dump with `reason = "logoutStart"`
and the empty-client dump 20 s later was skipped, the realm number was read from the GUID (`Piekasz-6842`) and the character's
history followed it to the new key. Not tried in the game: pressing the key binding, an instant logout in an inn or city.

- **Fixed**
  - Two characters with the same name on two realms shared one key when the client gives no realm name (Forever build
    71037): the second to log in replaced the first in the dump, and their history and auction prices mixed. The addon
    now takes the realm number from the player's GUID (`Player-<realm>-<id>`, read with `UnitGUID`) and saves the
    character as `Name-<realm number>`; the dump has `character.realmId`. A saved `Name-Realm` key of the same class and
    race still wins, and the dump saved under the bare name moves to the new key at login.
  - The server follows the key: a character with a realm in its key and no history yet takes the history kept under its
    former keys (the bare name, or the name with the old realm text) when their newest dump has the same class and race.
    The dumps are copied, the old directories and rows stay, and the long history imports them before the keep limit
    trims the copies.

- **Added**
  - A plain logout now leaves a fresh dump on disk. The addon takes a dump when the logout timer starts or the player
    chooses Exit (`PLAYER_CAMPING`, `PLAYER_QUITING`, both confirmed by `/mcp probe` in the real client on 2026-11-24),
    while the character is still loaded; the dump at `PLAYER_LOGOUT`, when the client is already empty, is skipped as
    before. The dump has `reason = "logoutStart"`. It only reads: nothing is reloaded and nothing is saved by the addon.
  - A long history database on the server: `history.db` (SQLite, the cgo-free `modernc.org/sqlite` driver) inside the
    history directory. Every dump the server reads adds one compact row per character (level, XP, gold, rested XP,
    played time, deaths, completed quests, zone) and the rows are kept beyond the `--history-keep` limit of full dumps.
    The dumps already kept in files are imported the first time a character is seen. Several server processes may share
    the file (WAL mode, rows keyed by character and dump time). The exe grows by about 4 MB.
  - The `eventLog` entries of every dump are kept in `history.db` (table `events`, unique by character, time, kind and
    quest, slot, zone or level), so the log outlives the addon's 300 entries / 14 days. A repeated `money` entry
    keeps its time and takes the latest copper. The domain package `game/eventlog` reads the section.
  - A save watcher: while the server runs it checks the dump file's modification time every 5 seconds and, when it
    changed, records the dump in the history and `history.db`, so a save between two tool calls is not lost.
  - Section `eventLog` (`Sections/EventLog.lua`): a bounded, timestamped list of level-ups, gold changes, quests accepted and
    turned in, zone changes and equipment changes, carried across saves (300 entries, 14 days), so one save holds the
    whole session with real times. Off with `/mcp off eventLog`.
  - A key binding "Save character state (dump and /reload)" in the game's Key Bindings, under WowMcp (`Bindings.xml`,
    `Keybind.lua`). It does what the panel's Save button does, on one key the player picks; no key is bound by default.
  - `/mcp probe` also reports whether the client knows the logout events `PLAYER_CAMPING`, `PLAYER_QUITING`,
    `PLAYER_LEAVING_WORLD` and `LOGOUT_CANCEL` (`probe.eventCandidates`).
  - A PayPal donation link next to GitHub Sponsors: the repository's Sponsor button (`.github/FUNDING.yml`), the npm
    package (`funding`, shown by `npm fund`), two README badges, and one line in the README, `SUPPORT.md` and the npm README. Nothing in
    the addon or the server changes, and the game shows no donation message.
  - Auction prices are kept in `history.db` per realm (table `prices`: realm from the `Name-Realm` key, item = the name
    the price is saved under, so a random suffix is its own item; time, lowest buyout and bid; each observation once).
    Every dump adds each price's latest observation and the visits it carries in `seen`; prices without a time or a
    buyout are not stored. The `wow_auctions` trend uses the longer of the dump's visits and the stored ones (one visit
    = observations under 10 minutes apart) and then says since when (`over N visits since <date>`).
- **Changed**
  - The feature request form on GitHub is now "Suggest a feature": it asks which game version you play, gives an example,
    says up front that only read-only ideas can be accepted, and asks to vote with a 👍 on existing suggestions. The README
    and `SUPPORT.md` link straight to it.
  - `wow_timeline` reads the event log in `history.db`: a level-up or a turned-in quest the log recorded gets the logged
    time (`exact: true`) instead of the time of the dump that showed it, and is listed once; logged level-ups and
    turned-in quests no pair of kept dumps shows (older than the kept dumps) are added. The other kinds still come from
    comparing dumps. A log that cannot be read leaves the dump times (`eventLogError`).
  - The number-only history answers read `history.db` before the oldest kept dump, so they are no longer limited to the
    last `--history-keep` dumps: `wow_gold_history`, `wow_progress` (overall pace, level-ups, sessions), the goal ETA
    rates and the earlier sessions `wow_session_rating` compares with. Answers that need the dump itself (`wow_changes`,
    `wow_journal`, consumption in `wow_ammo`, `wow_reagents`, `wow_shopping_list`) still use the kept
    dumps. When a stretch starts before them, `xpSources` says from which dump it counts (`countedFrom`, `partialNote`).
    A `history.db` that cannot be read leaves the answers as they were.

## 0.19.2 (2026-11-24)

Auction rows with their real stats, for the addon and the server. The server expects addon 0.19.2. Run in the real client on 2026-11-24 (no section errors, dump 44.1 ms): 89 gear rows from five searches, every one with its tooltip, suffix variants as separate entries with their stats, five trinkets saved.

- **Fixed**
  - Auction gear with a random suffix ("of the Eagle", "of the Monkey") was saved under the base item's name with the base
    item's stats, which for such items is no stats at all. A browse row is an item key (item, level, suffix): each suffix is
    now its own `auctions.prices` entry, named as the auction window names it, with `suffix`, the suffix's `statTokens` and
    the row's `tooltip` lines (the link's full tooltip when it names the same item, so Equip / Use effects are visible too). A listing the player opens is
    read from its own link.
  - A browse row whose item data the client had not loaded yet was dropped (a whole trinket search could vanish) or kept
    without level and stats. The addon now asks the client to load that item (`C_Item.RequestLoadItemDataByID`, an item
    cache request, never an auction house query) and reads the waiting rows again when the data arrives, at most 8 times
    per search. Wait a second after a search before `/mcp reload`.
- **Changed**
  - `wow_auctions` passes on `suffix`, `tooltip`, `speed`, `damageMin` and `damageMax`, and marks a gear price saved
    without its tooltip (an older addon) with `statsNote`: its stats may be the base item's only.

## 0.19.1 (2026-11-23)

Internal quality work after a design review, for the addon and the server. No tool, prompt or dump field changes, except the
one noted under "Changed". The server expects addon 0.19.1.

- **Changed**
  - Item quality names come from one table (`itemkind.QualityName`). Bag plan and session journal answers now name
    Artifact and Heirloom items instead of printing the quality number 6 or 7.
  - The dump history is recorded by an explicit step after the dump is loaded (`mcpserver`), no longer as a side effect of
    `storage.Store.Load`. Nothing changes for the player: every tool call and `--check` still record the dump.
  - Every domain package (`server/internal/game/*`) now reads the dump into its own types in `read.go`, decides on
    those types and builds the answer in `render.go`. Untyped maps outside those two files went from 642 to 0, and
    no function is over 80 lines. Tool answers are byte-identical on all test dumps (golden tests unchanged).
    Typed readers pass through only the fields they know: a field a newer addon adds to talents, Legacy perks or
    similar typed sections shows up in `wow_section` at once, and in the dedicated tool after the server learns it.
  - The tool registry is split into groups (status, character, windows, shopping, questing, progress, goals, account).
  - Addon: the logout guard (`LogoutGuard.lua`), the carried loot and XP periods (`Periods.lua`) and the Legacy API
    discovery scan (`LegacyDiscovery.lua`) moved out of `Snapshot.lua` and `Sections/Legacy.lua` into their own files.
    The dump is byte-identical. New addon files need a full game restart, not only `/reload`.
  - Addon: the scan for the client's Legacy API names (`legacy.discovery`) no longer runs on every first dump of a
    session, where it cost 70 to 140 ms in the real client (a normal logout included). It runs only when no perk tree
    was found, or after `/mcp probe`. `wow_legacy discovery=true` says so (`discoveryNote`) when the dump has none.
- **Added**
  - `CONTRIBUTING.md` spells out the code rules (typed domain, rules apart from presentation, no hidden side effects,
    one place for shared lookups, size limits), and `server/architecture_test.go` checks them: domain imports, untyped
    maps per domain package and functions over 80 lines against baselines that may only go down, addon files over 350 lines.
- **Fixed**
  - The golden tests no longer fail on a Windows whose system language is not English (the "file not found" text).

## 0.19.0 (2026-11-23)

From this release the addon and the server share one version number, and one `v<version>` tag publishes everything: the
GitHub release, the npm packages and the addon on CurseForge. Earlier entries below keep their separate numbers.

### Server 0.19.0

- **Changed**
  - Expects addon 0.19.0 (`wow_doctor` and `wow_status` suggest the update to an older addon).

### Addon 0.19.0

The changes below were prepared as addon 0.18.0, which was never released under that number.

- **Added**
  - `/mcp quest`: with the TomTom addon installed, every "zone x, y" in the answer is a chat link; a click sets a TomTom waypoint
    (with its arrow) titled with the NPC or object and the quest, e.g. "Barithras Moonshade (Cave Mushrooms)". The map ID comes from
    the zone data QuestieDB ships (the same area-to-map table and overrides Questie uses for its map pins), or from the zone name
    through the client's map data. Only a click sets a waypoint; a shift-click keeps the client's default.
  - `/mcp quest` also lists the objective targets (`objectives:`) between the start and the end.
- **Fixed**
  - The `group` section failed in a real party on Forever ("attempt to perform boolean test on ... a secret boolean value"):
    `UnitInRange` returns its "range checked" value as a secret there. Secret values are now dropped before any test, so the
    group is saved and `inRange` is left out when the client hides it.
- **Changed**
  - TomTom is an optional dependency (`## OptionalDeps`), never required: without it, or in a client without
    `LinkUtil.RegisterLinkHandler`, the coordinates stay plain text, and the first lookup with coordinates in a session suggests TomTom once.

## Addon 0.17.0, server 0.18.0 (2026-11-22)

Fixes from the first real Legacy dump (addon 0.16.0, Forever build 71037): the perk trees were read (27 perks, ranks,
requirements, the shared point currency), but without tree names, with most descriptions missing, and the section took
78 ms of a 112 ms dump.

### Addon 0.17.0

- **Fixed**
  - A spell tooltip with only one line under the name (the spell type, e.g. "Passive", before the client loads the spell
    data) is no longer saved as a Legacy perk description; `C_Traits.GetTraitDescription` is tried first, the spell data is
    requested and `missingDescriptions` counts the gap, so the next dump has the text. Talent descriptions are unchanged.
  - The Legacy discovery scan of `_G` runs once per session and again only after the client loads an addon with "Legacy" in
    its name (the load-on-demand Legacy window), not on every dump.
- **Added**
  - `legacy.configs[].trees[]`: `nodes` (perk count) and `info` (plain fields of `C_Traits.GetTreeInfo`).
  - `legacy.discovery`: `C_*` namespaces with "Challenge" in the name, `members` (functions named for Legacy or challenges
    inside any `C_*` namespace, as `C_X.Name`), the argument-free readings of both, and `children` (names of table fields)
    of Legacy frames, to find where the client keeps the Legacy challenges: the second real dump showed no achievement
    category, no `C_*Legacy*` namespace, only the UI functions `ShowLegacyChallenge` / `ToggleLegacyChallenges` and the
    frames `LegacySystemFrame` and `LegacyChallengeObjectives`.

### Server 0.18.0

- **Fixed**
  - `wow_legacy` names the trees Professions, Adventure and Resourcefulness from their trait tree IDs (1187, 1188, 1189,
    verified in the real dump) when the client gives no names, and picks the Legacy config by those IDs.
  - Points come from the one currency the trees share (the real client repeats it per tree): `unspent`, `spent` and `max`
    (renamed from `characterCap`: the points this character can have now; `meaning` explains the assumption).
- **Added**
  - `unrevealed: true` on the "?" perks the game calls "Unknown", and a note not to recommend them.
  - Expects addon 0.17.0.

### Repository

- CurseForge: a release tag packages the addon (`.pkgmeta`, BigWigsMods/packager) and uploads it with the
  `CurseForge` workflow; running the workflow by hand builds the zip only.

## Server 0.17.1 (2026-11-22)

- **Fixed**
  - npm: on macOS and Linux `npx -y @piekasz/wow-mcp` failed with `EACCES` because the server binary in the platform
    package had lost its executable bit. The launcher now restores it before starting the server.
- **Docs**
  - README badges: CI status, latest release, npm version, license and the technologies used.

## Addon 0.16.0, server 0.17.0 (2026-11-22)

First public release. The addon is renamed so the project does not depend on any one AI vendor, and the server can be
installed with `npx`.

### Addon 0.16.0

- **Changed**
  - The addon is now called **WowMcp** (folder `addon/WowMcp`, `WowMcp.toc`, saved variables `WowMcpDB` in
    `WTF/.../SavedVariables/WowMcp.lua`; it had a working name before). The old saved variables are not carried over: after
    installing, take one dump with `/mcp reload`. Remove the old addon folder from `Interface/AddOns`.
  - Slash commands: `/mcp` (was `/cb`) and the alias `/wmcp`. If another addon also registers `/mcp`,
    use `/wmcp`.
  - Addon messages no longer name a specific assistant ("Note saved").

### Server 0.17.0

- **Changed**
  - Reads `WowMcp.lua` and the `WowMcpDB` root (the file of the renamed addon). Answers point to `/mcp` commands.
  - Tool, prompt and server texts name no specific MCP client; the `first_run` prompt is titled "First time with wow-mcp".
- **Added**
  - npm packages: `npx -y @piekasz/wow-mcp` runs the prebuilt server for the platform (package `@piekasz/wow-mcp` plus one
    `@piekasz/wow-mcp-<os>-<arch>` package per platform, chosen by npm), and `npx -y @piekasz/wow-mcp install-addon` copies the bundled addon
    into the game (game folder from `--wow-dir`, `WOW_DIR`, the Windows registry entry of Battle.net or the usual install
    folders on every drive, e.g. `D:\Games\World of Warcraft`; client folder from `--client`, `WOW_CLIENT` or the only one present). Under npm the dump history lives in the user
    data directory so it survives updates. `scripts/npm-pack.sh`, `scripts/npm-publish.sh`, and a release workflow job
    that publishes them when the `NPM_TOKEN` secret is set.
- **Removed**
  - The `install.ps1` option that edited one specific client's configuration. The installers now print a generic
    `mcpServers` entry for any MCP client.

### Repository

- Community files: `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md` (Contributor Covenant 2.1), `SECURITY.md` (private vulnerability
  reports), `SUPPORT.md`, `CODEOWNERS`, `FUNDING.yml` (GitHub Sponsors), issue forms for bugs and feature requests, a pull request template; `main` is protected and changes go through pull requests.
- The npm release job can publish with npm trusted publishing (no stored token) or with the `NPM_TOKEN` secret.
- CI runs on GitHub for the first time: the workflow syntax is fixed, `.gitattributes` keeps LF line endings on every OS
  (golden files are compared byte for byte), and tests that assumed Unix paths now pass on Windows.
- README and docs describe any MCP client instead of specific products, with an `npx` quick start.
- Local-only files (personal notes, internal review notes) are no longer tracked.

## Addon 0.15.0, server 0.16.0 (2026-11-22)

Reading the Forever Legacy system (Legacy Points, the Professions / Adventure / Resourcefulness perk trees, Legacy challenges), so
The assistant can advise where to spend Legacy Points without screenshots. No public source names the client API behind Legacy, so this
version reads it in a way that does not depend on one name. **Not yet checked in a real client.**

### Addon 0.15.0

- **Added**
  - Section `legacy` (every dump): every `C_Traits` config that is not the class talents, read like the talents with ranks,
    descriptions (also `C_Traits.GetTraitDescription`), arrows (`requires`), points gates (`locked`, `requiresSpent`) and the trait
    currency with the linked account currency (unspent, spent, cap, total earned); achievements in the "Legacy" category as
    `challenges`; and `discovery`: the client's `Enum`, `C_*` namespaces, functions and frames with "Legacy" in the name, with the
    results of their argument-free `Get`/`Is`/`Has`/`Are` functions. Only reads: nothing learns, applies or resets a perk, and global
    functions are never called (the harness enforces it).
- **Changed**
  - Talent tree reading moved to the new foundation file `TraitTree.lua`, shared by `talents` and `legacy` (talent dumps unchanged).

### Server 0.16.0

- **Added**
  - Tool `wow_legacy`: points (unspent, spent, per-character cap, account total), the perk trees with status `available` /
    `locked` / `maxed`, what each perk requires, `canLearnNow`, open and completed challenges; `tree` filter and `discovery=true`.
    When no perk tree was found it explains what to open in game and shows `discovery`.
- **Changed**
  - Expects addon 0.15.0.

## Addon 0.14.1, server 0.15.1 (2026-11-22)

Found when addon 0.14.0 first ran in the real client (Forever build 71037).

### Addon 0.14.1

- **Fixed**
  - The character key no longer loses the realm. Since client build 71037 `GetRealmName()` returns an empty string, so a character
    was saved as `Name` instead of `Name-Realm`: its history split, and another character with the same name on a different realm
    would have overwritten it. The key now also tries `GetNormalizedRealmName()` and then keeps the one saved `Name-Realm` key
    whose dump has the same class and race; with none or several such keys it stays the bare name, as before. The key is fixed for
    the session. At login a dump saved under the bare name in the meantime moves to the realm key (the newer dump wins).
  - `guide.next` reaches the current step: at most 5 open steps before the furthest done step are listed (they used to fill all
    15 places for a player who skipped part of a guide). New field `guide.passedSteps` counts all of them.

### Server 0.15.1

- **Changed**
  - `wow_guide` reports `passedEarlierSteps` and explains that the steps after the passed ones are where the player is now.
  - Expects addon 0.14.1.

## Addon 0.14.0, server 0.15.0 (2026-11-21)

Found in real use: the assistant sent the player to Ratchet to turn in The Glowing Shard, because the quest text and Classic knowledge say
so. In World of Warcraft: Forever the quest ends at Falla Sagewind above the Wailing Caverns, and the player's own addons (Questie's
database, the TUGs guide in Guidelime) already had the right data. Now the assistant reads that data instead of guessing.

### Addon 0.14.0

- **Added**
  - Section `questDb`: for every quest in the log and every quest of the next guide steps, who starts it, who ends it and what the
    objectives are (NPC, object or item), with the zone and map coordinates. Read in game from the quest database the player has
    installed (QuestieDB, the data Questie shows on the map) through its public read functions, with that addon's own corrections
    applied. WowMcp ships no quest data; without QuestieDB the section is absent. New foundation file `QuestDb.lua`.
  - Section `guide`: the leveling guide the player follows in Guidelime (for example TUGs), how many steps are done and the next
    15 open steps in guide order. Steps for other races or classes are left out. Quest steps count as done from the quest log
    (Guidelime does not store those), other steps when the player ticked them or a later step is done. The guide text is seen when
    the guide addon registers it (`hooksecurefunc`); Guidelime's own data is never changed.
  - `/mcp quest <ID, exact quest name or NPC name>`: prints where the quest starts and ends in chat and stores the answer in the
    dump (section `questLookups`, last 10), so the assistant can also answer about quests outside the log.
  - `## OptionalDeps: QuestieDB, Questie, Guidelime` in the `.toc`, so those addons load first when installed.

### Server 0.15.0

- **Added**
  - `wow_quest_lookup` (`quest`: ID or a fragment of a quest or NPC name; empty = the quest log): pick-up, objective and turn-in
    places with coordinates, the quest's state, the next quest of the chain, the direction and distance from the character when it
    stands in the same zone, and `nextStep` as a one-line answer ("Turn in at Falla Sagewind (The Barrens 48.2, 32.8)"). When the
    data is missing the answer says what to install or type in game and tells the assistant not to answer from memory.
  - `wow_guide`: the next steps of the player's guide as readable text (quest tags become quest names), each quest with its state and
    the place to go.
  - The server instructions tell the assistant to use these tools for quest locations instead of memory.
- **Changed**
  - `ExpectedVersion` of the addon is 0.14.0. 51 tools.

**Addon side run in the real client on 2026-11-22; server tools not yet** at the time of writing (mocks, golden tests and unit tests; the QuestieDB values for The Glowing Shard
were checked against the installed database: quest 6981 ends at NPC 8418, The Barrens 48.2, 32.8).

## Addon 0.13.0, server 0.14.0 (2026-11-16)

**Not yet run in the real client** (mocks and tests only). Found in real use: the player browsed bows in the auction house and
asked "which bow should I buy?", but `auctions.prices` held only the price, so the assistant could not compare the offers with the
equipped bow (no DPS, required level, type or slot).

### Addon 0.13.0

- **Added**
  - Price entries of gear now carry item details: `type`, `subType`, `classID`, `subclassID`, `equipLoc`, `itemLevel`,
    `requiredLevel`, `quality`, `sellPrice`, `bindType`, `statTokens` and, for weapons, `dps`. Both auction paths store them
    (the modern `C_AuctionHouse` results and the classic list), through the same item helper as the vendor and equipment
    (`Items.addCachedDetails`). Only what the client already holds: `GetItemInfoInstant` (static tables, never a server query)
    always, and `GetItemInfo` / `GetItemStats` only after `C_Item.IsItemDataCachedByID` says the item is in the local cache (when
    the client has no such function, only the static fields are stored). No `Item:ContinueOnItemLoad`, no
    `RequestLoadItemDataByID`, no tooltip. An item that is not cached keeps the price and gets only the static fields; a later visit
    fills in the rest and an earlier visit's details are never erased.
  - Size: no tooltip text (so no `speed` or damage range, `dps` comes from the stat tokens), no localized `stats` labels, and
    details only for gear (equippable items): a cached weapon entry grows by about 340 bytes, so at the cap of 400 prices the
    section grows by about 135 kB (about 100 kB for 300 prices); details for every price would be about 165 kB at the cap, over the
    150 kB budget. Non-gear prices stay as before.
- **Tests**: the harness has cached and uncached items for both auction paths (modern browse results and the classic list),
  fails when `GetItemInfo` / `GetItemStats` is called for an uncached item, and checks that non-gear and unknown items stay plain
  and that no price has a tooltip.

### Server 0.14.0

- **Added**
  - `wow_auctions` returns the details per price and, for equippable items, `vsEquipped` (the same comparison as `wow_vendor`),
    `canAfford` (lowest buyout against the character's money), `levelOk` and `qualityName`. New filters like `wow_vendor`:
    `type` (also matches a plural subtype: "Bow" finds "Bows"), `affordable`, `usable`, `equippable`; with `equippable` the list is
    sorted by the best upgrade first (`sortedBy`). An item the client had not cached has a slot but no numbers, so it gets no
    `vsEquipped`. Old dumps without details behave as before and `detailsNote` says to update the addon, browse again and
    `/mcp reload`.
  - The `gear_upgrades` prompt carries `auctionUpgrades` (up to 10, best first) and `what_to_auction` says that gear with an
    upgrade is something to buy, not to sell.
- **Changed**
  - The vendor and auction judgement share one implementation (`items.Buyer`: `Admits`, `Judge`; `MatchesType`, `UpgradeScore`).
  - A candidate without a tooltip but with its own DPS or armor numbers (a price entry) is now compared on them; before, DPS and
    armor were skipped without a tooltip. `wow_vendor` answers are unchanged.
  - `wow_vendor type` also matches a plural subtype ("Bow" and "Bows").
- **Tests**: bows on the auction house versus the equipped bow, filters, an old dump, `items.Buyer`; golden files for the new
  `wow_auctions` arguments.
- Expected addon version is 0.13.0.

## Addon 0.12.0, server 0.13.0 (2026-11-16)

**Verified in the real client (addon 0.11.0, server 0.12.2, 2026-11-16):** dump 26.1 ms with no section errors; the garbage
dump at the exit from the game was skipped by the history; the vendors history with class IDs; the modern auction house API
returned a commodity price (Bronze Tube); `wow_doctor` gave 11 of 12 checks ok. Not yet seen in the real client: the bank, the
stable, friends, recipes and a fresh flight map. The run exposed the two defects fixed here. **Addon 0.12.0 and server 0.13.0
are not yet run in the real client** (mocks and tests only).

### Addon 0.12.0

- **Fixed**
  - Loot and XP were lost when the game saved twice before the server read the file. `loot` and `xpGains` start from zero after
    every save, and the server reads the file only when a tool is called; in the real run the player played for hours
    (level 22 to 23), typed `/mcp reload` (a save with the whole session's loot) and logged out 10 s later (a save with
    `loot.items = []`), so the first save was overwritten before the server ever saw it. The addon no longer assumes the server
    read a dump: when a login closes a period that has content, the period (its fields plus `until`, the `dumpedAt` of the save
    that closed it) is appended to the section's new `periods` list, oldest first, and carried on from dump to dump. The list is
    bounded: at most 20 periods (`Snapshot.MAX_PERIODS`) and none older than 7 days (`Snapshot.MAX_PERIOD_AGE`); loot items per
    period keep the 100-item cap; empty periods (no own loot; no XP from messages or the bar) are not stored. The current-period
    fields keep their shape and meaning, so older servers keep working. A skipped garbage logout dump continues the period
    and keeps its `periods`.
  - `flightPaths.nodes[].zone`: a position on the flight map that only resolves to a continent or above (`mapType` below
    `Enum.UIMapType.Zone`) is no longer written as the zone; a continent goes to the node's new `continent` field. The
    real dump's `zone = "Eastern Kingdoms"` on every point came from the flight map captured by addon 0.7.0; the current addon
    takes zones from the zone maps first, so `zone` stays `zone`.
- **Tests**: the `lifecycle` harness mode covers the real sequence (save, save, then read: nothing lost), the count and age
  bounds (with garbage entries), a garbage logout dump in between, that empty periods are not stored, that everything carried
  passes the sanitizer unchanged, and that a continent-only position becomes `continent`. `relog` can continue the game clock.

### Server 0.13.0

- **Fixed**
  - `wow_changes`, `wow_journal` (and so `wow_session_rating` and the `week_in_game` prompt) and `wow_progress` (`xpSources`)
    merge the carried loot and XP periods (new package `game/periods`): the gain after the older dump is the current periods of
    the newer dumps plus every carried period that started at or after the older dump's `dumpedAt`. A carried period with the
    same `since` as a current period already counted is skipped, and carried periods are de-duplicated by `since` (keeping the
    latest `until`), so a period seen in several history snapshots counts once. The answer says `periodsMerged` (with
    `periodsNote`) and `since` is the start of the earliest merged period, so the `partialNote` disappears when the carried
    periods cover the gap. Dumps without `periods` give exactly the previous results.
  - `wow_flights` and `wow_quest_route` matched no zone with the real dump although the character knows 18 flight points: every
    point had the continent as its `zone`. A zone field that is a continent name now counts as no zone (`Node.Continent`), and a
    point whose zone field or map ID matches no quest zone is matched by its name like points from older dumps (the match says
    `zoneFieldUnmatched`). With the real dump: Westfall, Redridge Mountains, Duskwood, Loch Modan, Wetlands, Dun Morogh and
    Stormwind City have a flight point; Darkshore (another continent) does not, and the existing note explains why.
  - `wow_section` with `limit` and no `path` returned all 77 dungeon activities: the only key of `dungeons` is a list. `limit`
    now also cuts the lists directly inside the object, and `limited.lists` says each list's `total` and `shown`.
- **Tests**: `game/periods` (old dumps, merge after the baseline, de-duplication across history snapshots, a continued
  period); the real sequence on the real dump plus a synthetic carried period in `progress` (nothing lost, nothing counted twice)
  and `journal`; `wow_flights` on the real dump; `limitValue` and `wow_section` one level down. New fixture `fixtures.Real0110`.
- Expected addon version is 0.12.0.

## Addon 0.11.0, server 0.12.2 (2026-11-16)

An options page for the addon, and two follow-ups from the code review (`docs/CODE_REVIEW_2026-11-16.md`, "Recommended, not done").
**Not yet run in the real client** (mocks and tests only): in particular, which options API Forever 1.60.1 really offers is unconfirmed.

### Addon 0.11.0

- **Added**
  - An options page in the game menu (Interface / AddOns), `Options.lua`: the same switches as the `/mcp` commands (the minimap
    button, compact panel, auction house price in the tooltip, played time, and every section that can be turned off). It calls
    the existing setting functions, so the page and the commands cannot disagree; it never saves a dump and does nothing in the
    game. It supports the modern `Settings.RegisterCanvasLayoutCategory` API and the old `InterfaceOptions_AddCategory`; it is
    registered at login and drawn on first show, all inside `pcall`. When the client has neither API (or one that raises an
    error) there is simply no page and `/mcp` works as before.
  - `/mcp options` opens the page when the client allows it, otherwise it says so.
- **Fixed**
  - The logout guard against the dump of an unloaded client no longer depends on `/mcp off`: a disabled sign section (stats,
    spells, equipment, bags) is read for the check only and never stored, so a player who turned off three of them no longer
    loses the protection of the good snapshot at an exit from the game.
- **Tests**: the harness `lifecycle` mode exercises the page on the modern API, the old API, an API that raises and no API,
  and the guard with the sign sections turned off.

### Server 0.12.2

- **Fixed**
  - MCP prompts are now under the response cap (`--max-response-kb`), like tools and resources. An oversized prompt keeps its
    instruction and rules, and only the embedded JSON data becomes the usual truncation envelope.
- Expected addon version is 0.11.0.

## Addon 0.10.1, server 0.12.1 (2026-11-16)

Fixes from an independent review of the whole codebase before the international release
(`docs/CODE_REVIEW_2026-11-16.md`). No new features. **Not yet run in the real client** (mocks and tests only).

### Addon 0.10.1

- **Security**
  - Every write to SavedVariables now ends with one sanitizing pass (`Compat.sanitizeTree`, run at the end of every dump and
    once more at logout): secret values and keys, functions and tables nested deeper than 20 levels are removed, and a list
    stays a list. Before, several values reached the saved dump without `plain()`: the vendor, trainer and flight master NPC
    name (`UnitName("npc")`, which can be a secret), trainer service names, costs and level requirements, the pet name and
    the raw container item fields. Those reads now also go through `plain()` at the source.
- **Fixed**
  - `Compat.firstValue` turned a boolean `false` into `nil`, so `group.members[].online` was never `false` and the server
    could not flag offline group members.
  - The loot list could grow past 100 items when it held entries without `lastAt` (the trim raised an error after the
    new item was already added).
  - `/mcp note` cut the text at 500 bytes, possibly in the middle of a UTF-8 character; it now cuts on a character boundary.
  - A Polish comment in `AuctionTooltip.lua` (the product is English only).
- **Tests**: the `lifecycle` harness mode checks the sanitizing pass (secrets, functions, depth, lists), `firstValue`
  with `false`, the note cut and the loot limit; a secret vendor name is now asserted *not* to reach the dump.
  `scripts/check-addon-fixtures.sh` (and so `make test`) now also runs the `lifecycle` mode.

### Server 0.12.1

- **Security**
  - History directory names come from character keys in the dump: Windows device names (`CON`, `NUL`, `COM1`...),
    trailing dots and spaces and names over 100 bytes (cut, with a hash of the whole key) are now made safe too, besides
    separators and `.` / `..`.
  - Bounded reads: a dump file over 64 MB is refused before parsing, and a history snapshot is refused when its file or its
    decompressed `.json.gz` content is over 64 MB (a "decompression bomb" could exhaust memory before).
  - The game directory guard also applies to a dump given only with `--file`: the game directory is derived from the
    `WTF` folder in its path, and history, log and goals file paths inside it are rejected.
  - The response size cap (`--max-response-kb`) now also covers MCP resource reads (`wow://<character>/<section>`
    returned whole sections, e.g. the 60 kB `probe`, uncapped).
  - `wow_goal_set` input limits: numeric targets up to 1,000,000,000 (a huge gold target overflowed the copper amount),
    names up to 100 characters, at most 200 goals in the file.
  - The Lua parser rejects a decimal escape over `\255` (it wrapped around silently).
- **Fixed**
  - Concurrent requests: two `wow_goal_set` calls at the same time could lose one goal (read, change, write without a
    lock); now serialized. Concurrent history saves shared one `<dumpedAt>.json.tmp` file and failed with "rename: no such
    file"; temporary files now have unique names and saves are serialized.
  - `snapshot.AsInt` converted NaN, infinities and floats outside the int64 range to arbitrary numbers; they are now "not a
    number". `snapshot.Money` wrote negative amounts as `-50c` for -1s 50c; it now writes `-1s 50c`.
  - A relative `--history-dir` is resolved from the server directory (like `--log` and `--goals-file`), not from the
    working directory the MCP client happens to start the server in.
  - Tool descriptions that were no longer true: `wow_buffs` said the addon does not save aura durations (it does since
    0.7.0), `wow_flights` said flight points have no zone (they have since 0.7.0, and map IDs since 0.10.0). The
    `timingNote` text of `wow_buffs` was corrected the same way.
- **Changed**
  - One rounding helper, `snapshot.Round1`, instead of five copies (`consumption.Round1` and local `round1` in `rested`,
    `sessionrating`, `shopping`, `questrewards`; `rested` rounded negative values towards zero).
  - Every source file carries the SPDX licence identifier and the author line (`NOTICE`, point 4).
- **Install scripts**: `install.sh` and `install.ps1` refuse to mirror into an addon folder that is a symlink or junction
  (the mirror deletes files, which through a link would be outside the AddOns folder); `install.ps1` validates `-Client`
  as a single folder name; `install.sh` compares the install directory with the game directory by physical paths, also
  when the install directory does not exist yet.
- **Tests**: path traversal and Windows names in the history, the decompression bomb, oversized dump and history files,
  concurrent history saves, concurrent tool calls with goal writes (`go test -race ./...` is clean), the resource cap,
  resource URIs that try to read meta fields or other paths, hostile parser inputs, goal limits, `AsInt` and negative money.

## Addon 0.10.0, server 0.12.0 (2026-11-16)

More locale-independent data, a diagnostic tool and better behaviour in MCP clients. **Not yet run in the real client**:
the new addon fields are tested on mocks only (API names chosen from the real `/mcp probe` result).

### Addon 0.10.0

- **Added**
  - Map and instance IDs: `quests[].zoneMapID`, `flightPaths.nodes[].nodeID` and `.zoneMapID`, `dungeons.activities[]`
    `categoryID` / `groupID`, `dungeons.lfg[]` `typeID` / `groupID` / `mapID`, `dungeons.lockouts[]` `instanceID` /
    `difficultyID`. New file `Maps.lua` resolves zone names to map IDs through `C_Map`, in the client's own language.
  - Weapons carry numbers: `speed`, `dps`, `damageMin`, `damageMax`, read with the client's own format strings
    (`DAMAGE_TEMPLATE`, `DPS_TEMPLATE`), so they work on any client language.
  - `spells[].icon`, `spells[].selfBuff` and `auras[].icon`.
  - `/mcp probe` also lists the achievement API (`C_AchievementInfo`, `GetAchievement*`, `GetCategory*`, `GetStatistic*`), so
    the next probe from the game shows whether Forever has achievements at all.

### Server 0.12.0

- **Added**
  - `wow_doctor`: one diagnostic report with ordered checks (`ok` / `warn` / `fail`, detail and fix): dump file, account,
    dump age, addon version, section errors, dump speed, disabled sections, windows never opened, unloaded dump, client
    language, history, goals file; plus `summary` and `nextStep`. Package `game/doctor`.
  - Tool annotations for MCP clients: every tool is marked read-only and idempotent with a title, except `wow_goal_set`
    (writes the goals file). Clients can auto-approve the read-only tools.
  - Response size cap: `--max-response-kb` (default 48, 0 = off). An answer over the limit becomes an envelope with
    `truncated`, sizes, a `hint` naming the tool's own arguments, a `topLevel` size summary and a shortened `partial`
    answer, instead of flooding the model's context.
  - `wow_section`: `path` (dot path into the section) and `limit` arguments for paging through big sections such as `probe`.
  - Several accounts: `--account` / `WOW_ACCOUNT` pins an account folder; without it the newest dump wins and
    `wow_status` / `wow_doctor` say which account was chosen and which others exist. A character name used on several
    realms now gives a clear error listing the full `Name-Realm` keys.
- **Changed**
  - `wow_flights` and `wow_quest_route` match quest zones to flight points by map ID first, by name only for older dumps.
  - `wow_dungeons` shows `id`, `mapID` and `locked`; the English-only note appears only when the classic table was needed.
  - `wow_compare_item` and other comparisons use the numeric weapon fields; the English tooltip parse is only a fallback.
  - `wow_buffs`: an aura counts when its spell ID or icon matches; new `selfBuffsNotActive` for non-English clients. The
    "one Aspect is enough" grouping still relies on English spell names (the response says so).
- **Documentation**
  - `docs/PUBLISHING.md`: a checklist of what the owner has to do to publish the project (repository, first tag, signing,
    registries and addon sites).
- **Decided**
  - Smaller dump (roadmap 31) measured and not implemented: in the real dump repeated item detail is 3.5% of the file
    (all tooltips together 7.3%), far below the 25% that would justify an item dictionary and a schema change.
  - Achievements (roadmap 27) wait for a probe from the game: the first probe did not ask for the achievement API, and
    the addon does not guess function names.

## Server 0.11.1 (2026-11-16)

- **Added**
  - License: **GPL-3.0-or-later** with an additional term under section 7(b) of the GPL: the author attribution
    (Piekasz, GitHub: Piekario) must be kept in every copy and modified version. Files `LICENSE` (the GPL text) and
    `NOTICE` (copyright, the attribution term, trademark note); the README, the addon `.toc` (`## Author`) and the main
    source files name the author. Release archives include both files.
  - `TASKS.md`: a checklist view of the roadmap, generated by `python3 scripts/tasks.py` (`--check` fails when it is out
    of date). The roadmap got section 11, "International release".

## Addon 0.9.0, server 0.11.0 (2026-11-16)

The server is meant to work on every game client language (deDE, frFR, esES, ruRU, koKR, zhCN...), not only English.
Until now some logic read English text from the client (item types, tooltip lines, stat labels, profession names), and
on a localized client those features quietly stopped working. **Not yet run in the real client; the localized-client
behaviour is tested only on mocks** (the new deDE harness mode and fixture).

- **Added**
  - Addon: locale-independent identifiers next to the existing text fields (nothing removed): `character.locale`
    (`GetLocale()`); for items `classID`, `subclassID`, `bindType` (returns 12-14 of `GetItemInfo`, `GetItemInfoInstant`
    as a fallback), `statTokens` (the `GetItemStats` keys such as `ITEM_MOD_STAMINA_SHORT`, next to the translated
    `stats`), `quality` for items that had none (vendor, quest rewards); for bag and bank items `isBound` (container item
    info), `isQuestItem` and `questID` (`GetContainerItemQuestInfo`; `questID` = the item begins that quest); `skillID` for
    professions (`GetProfessionInfo`, and for the old Skills window a match against the client's own skill line names);
    `petAction` on pet spellbook entries that are commands or stances (`Enum.SpellBookItemType.PetAction`).
  - Server: `game/itemkind`, one place that classifies items by `Enum.ItemClass` / subclass / `Enum.ItemBind` IDs
    (ammo and its kind, the ammo a bow, crossbow or gun fires, food, potions, bandages, Trade Goods kinds, quest items,
    quest starters, bound and tradable items) with the English text only as a fallback for older dumps.
  - Server: `snapshot.Locale`, `EnglishClient`, `EnglishOnlyNote` / `AddEnglishOnlyNote`. Heuristics that still read
    translated names add a `localeNote` when `character.locale` is not English: `wow_buffs` (class buffs by spell name),
    `wow_dungeons` (the table of classic dungeons), `wow_flights` (flight points without a zone field matched by name),
    `wow_compare_item` (weapon speed and, without `statTokens`, damage and armor from tooltip lines). A dump without
    `locale` counts as English.
  - Tests: harness mode `dede` (`python addon/tests/run_addon.py dede server/testdata/WowMcp_dede.lua`): the
    Forever mock with German client text (item types and subtypes, tooltip lines, stat labels, class, race, profession,
    weapon skill and some item names, rank texts, loot and XP format strings) and the same IDs; it checks that German
    chat is parsed and English chat is not. New golden scenario `WowMcp_dede` for the tools that used to depend on
    English text, and parity tests (`server/locale_parity_test.go`, `itemkind`, `altmail`, `professions`, `items`,
    `pet`, `buffs`) proving the same classification, binding, quest detection, stat comparison, reagent categories
    and mail suggestions as on English data.
  - Server: `--client` flag (and `WOW_CLIENT`) to read only one client folder of the game directory.
- **Changed**
  - Server: item classification in `wow_bag_plan`, `wow_reagents`, `wow_leftovers`, `wow_alt_mail`, `wow_auctions`,
    `wow_ammo`, `wow_shopping_list` and `wow_pet` (food in bags) uses the item class IDs, binding and quest flags first.
    `wow_alt_mail` matches professions to materials by skill line ID and Trade Goods subclass (Cooking now also matches
    the modern "Cooking" subclass, not only the classic "Meat"). Profession tools in `wow_bag_plan` are recognised by
    item ID (Skinning Knife, Mining Pick, Fishing Pole) or the Fishing Pole weapon subclass. Professions are told apart
    from other skills by `skillID`. The weapon skill in `wow_compare_item` is found by the weapon subclass and skill
    line ID. Stat comparisons use `statTokens` mapped to the English stat names (so weights such as `agility` keep
    working on any client). Pet controls are recognised by `petAction`; ability ranks by the number at the end of the
    rank text ("Rank 3", "Rang 3"). The `wow_bank` / `wow_vendor` `type` filter also matches English class names
    against `classID` ("Trade Goods" finds "Handwerkswaren").
  - Server: `wow_bag_plan` `limits` says bound items are known when the dump has binding data.
  - Server: the game directory is searched in every client folder (`_retail_`, `_classic_`, `_classic_era_`, `_ptr_`,
    `_forever_`... any direct `_name_` folder), the newest dump wins. When the exe directory is not writable, the
    history, goals file and relative `--log` path default to `<user config directory>/wow-mcp` (next to the exe as
    before when it is writable). The `--wow-dir` help shows a neutral placeholder.
  - Addon: the English XP and loot patterns apply only when the client has none of its own format strings (the loose
    English XP patterns used to run on every client).
  - Test data: the Forever mock returns `classID` / `subclassID` / `bindType` matching its type texts, stat tokens
    (DPS equal to what the tooltips give), `isBound` (Hearthstone bound), quest info and profession skill IDs.
    Comparisons of items whose mock tooltip has no damage line now get DPS from the stat token instead of a false
    `DPS: -8.4`, and vendor items have a quality (so `minQuality` filters them).
  - Versions: addon 0.9.0, server 0.11.0 (the server expects addon 0.9.0).
- **Fixed**
  - Server: the "no WTF/Account/<account>/SavedVariables/... file" error had the file name and the directory swapped.

### Project tooling

- **Added**
  - `Makefile` with `test`, `lint`, `build`, `dist`, `version` and `clean`; `scripts/dist.sh` cross-compiles the server for
    Windows, macOS and Linux (amd64 and arm64) and builds per-platform archives with the addon, the installer, the docs
    and `SHA256SUMS`. Versions in archive names are read from the code.
  - `install.sh` for macOS and Linux (Wine, Lutris, Proton): finds the game, copies the addon with a backup, installs the
    server outside the game directory, runs `--check`, prints the MCP client configuration; `--dry-run`, `--rollback`.
    Tested on Linux against a fake game tree; not tested on macOS.
  - `.github/workflows/release.yml` (builds and attaches the archives on a version tag) and a CI matrix on Linux, macOS
    and Windows. Neither has run on GitHub yet: the repository is not published.
- **Changed**
  - `README.md` rewritten for an international audience: quick start per operating system, configuration for
    MCP clients, supported clients and languages, building from source, contributing.
    Owner-specific paths were replaced with placeholders. License: to be decided by the owner.

## Addon 0.8.0, server 0.10.0 (2026-11-16)

The whole product is now in English, because the project is meant to be an international MCP server. No functional
changes. Not yet run in the real client.

- **Breaking**
  - Prompts were renamed: `plan_sesji` → `session_plan`, `zakupy_u_trenera` → `trainer_shopping`, `co_sprzedac` →
    `what_to_sell`, `ulepszenia_ekwipunku` → `gear_upgrades`, `podsumowanie_sesji` → `session_summary`, `przeglad_konta` →
    `account_overview`, `rozwoj_profesji` → `profession_progress`, `rozwoj_talentow` → `talent_planning`,
    `co_wystawic_na_aukcji` → `what_to_auction`, `gdzie_na_dungeon` → `dungeon_finder`, `przed_wylogowaniem` →
    `before_logout`, `pierwszy_raz` → `first_run`, `zwierzak` → `pet_care`, `tydzien_w_grze` → `week_in_game`.
  - Text values in tool outputs changed language, for example quest status (`ready to turn in`, `in progress`, `failed`),
    session rating labels (`faster`, `slower`, `similar`), route phases, reagent categories, timeline event texts and age
    formatting (`2 d 4 h`). The stale-data line starts with `WARNING:` instead of `UWAGA:`. JSON keys, tool names and tool
    arguments are unchanged.
  - Documentation files were renamed: `docs/FIRST_RUN.md`, `docs/HOW_TO_ASK.md`, `docs/TOS_COMPLIANCE.md`,
    `docs/DUMP_FORMAT.md`.
- **Changed**
  - Addon: every chat message, `/mcp help`, `/mcp status`, panel labels (the button is now **Save**), tooltips and code
    comments are in English. Errors of event handlers in the dump read `event NAME: message`. Commands and the dump
    structure are unchanged.
  - Server: tool and argument descriptions, server instructions, prompt texts, notes, hints, assumptions, errors, log
    and `--check` output and code comments are in English. Prompts ask the model to answer in the user's language and to
    keep in-game names in English. `wow_export` always uses English labels; its `english` argument is deprecated and ignored.
  - `README.md`, `CHANGELOG.md`, `ROADMAP.md`, `docs/`, `install.ps1` messages and the CI workflow are in English.
  - Versions: addon 0.8.0, server 0.10.0 (the server expects addon 0.8.0).

## Addon 0.7.1, server 0.9.1 (2026-11-16)

Addon 0.7.0 and server 0.9.0 were run in a real Forever 1.60.1 client on 2026-11-15: **zero section errors, dump
29 ms**. Confirmed in the game: aura timers (`expiresAt`, `duration`), `pvp` (without rank), `equipmentSets` (empty list), descriptions of
all talents, trainer, vendor and flight master windows, quests, currencies, play time. Broken or missing: the dump of an
"empty" client at game exit overwrote a good dump; no `vendors` / `trainers` maps and no buyback list; no `loot`
and `xpGains` sections; no `zone` in flight points; recipes, the auction house, the stable and the loot method went through functions and events
that Forever does not have (the `/mcp probe` result in `testdata/real/probe_2026-11-15_addon070.json`). The missing `guild` section was
correct: the character is not in a guild. The principle is unchanged: the addon only reads, does nothing on its own and asks the game server for nothing.

### Addon 0.7.1

- **Fixed**
  - Exiting the game: on a real exit `PLAYER_LOGOUT` arrives when the character is already released (health 0, empty
    spells, gear and bags). The addon counts four signals (`stats.healthMax` not > 0, no spells, no gear, bags
    without size) and with at least two it **keeps the previous dump** with `lastSkippedLogoutAt` added. When there is no previous
    one, it writes a new one with `clientUnloaded = true`. `/mcp reload` works as before; `/mcp status` says when the last dump
    was kept.
  - `vendors` / `trainers` and `buyback`: the NPC name is sometimes a "secret" (`issecretvalue`), and comparing a secret threw an error
    in the vendor handler; without `pcall` in event handling this also aborted the subsequent handlers of the same `MERCHANT_SHOW`
    (the buyback list). Now the name goes through `plain()`, the history write is in `pcall`, and every event handler runs
    separately (an error goes to `errors` as "event MERCHANT_SHOW: …", at most 10, no repeats).
  - `loot` and `xpGains`: the sections appeared only after the first recognized message. Now the period starts at
    `PLAYER_LOGIN` (`since` = time of the previous save) and the sections are in the dump right away, with zeros. After a skipped "empty"
    dump at exit, the next login continues the period (the same `since`). The cycle "manual dump → dump on
    `PLAYER_LOGOUT` → `/reload` → `PLAYER_LOGIN`" has a test.
  - `flightPaths.nodes[].zone`: the zone from the continent zone maps (`C_Map.GetMapChildrenInfo` + `C_TaxiMap.GetTaxiNodesForMap`)
    for the flight map and the character's continent; a position on the flight map is no longer taken for a zone when it points to the flight map itself.
  - Auction house: the events `AUCTION_HOUSE_BROWSE_RESULTS_UPDATED` / `_ADDED` instead of `BROWSE_RESULTS_*`, which Forever does not accept.
  - Recipes: Forever has only `C_TradeSkillUI`, so ingredients come from `GetRecipeSchematic` (basic ones, with name and count
    in bags), `itemId` from `outputItemID`, and the cooldown from `GetRecipeCooldown`.
  - Stable: active pets from `C_StableInfo.GetActivePetList` (field `active = true`) before the stabled pets.
  - Group: `C_PartyInfo.GetLootMethod` returns a number (`Enum.LootMethod`); the addon converts it to a name.
- **Changed**
  - Version 0.7.1 (`Core.lua`, `.toc`).
  - `/mcp probe` also checks `GetTitleText`, `GetQuestID`, `GetObjectiveText`, `CanMerchant*`, `GetRepair*`, `GetTaxi*`.
- **Added**
  - `lastSkippedLogoutAt` and `clientUnloaded` (dump meta).
  - `loot.messagesSeen`, `messagesParsed`, `messagesSecret` and the same in `xpGains`, plus `xpGains.barXp` (growth of the XP bar
    from `PLAYER_XP_UPDATE`, also across a level-up): diagnostics for when chat messages are secrets or do not match the pattern.
  - `flightPaths.taxiMapID`, `nodeFields` (fields of the client's raw entry) and `zoneSources` (how many zones from the node field, from the zone map, from the position).
  - `guild = { inGuild = false }` on every dump when the character is not in a guild (only `IsInGuild()` and `GetGuildInfo("player")`,
    with no request for the member list); `guild.inGuild = true` when a list is saved.
  - `stable.pets[].active`, `recipes.*.recipes[].reagents` for `C_TradeSkillUI`.
  - The Forever mock in the addon test matches the `/mcp probe` result (no classic recipe, auction, stable, PvP rank
    and loot method functions; rejected events) and there is a new mode `run_addon.py lifecycle` (the lifecycle from the real game).

### Server 0.9.1

- **Fixed**
  - The dump of an "empty" client (`clientUnloaded` or at least two signals: health 0, no spells, no gear, no bag sizes)
    does not enter the progress history: `wow_changes`, `wow_progress`, `wow_gold_history`, `wow_timeline`, `wow_journal` skip it
    and report the number skipped (`skippedUnloadedSnapshots`); the history files stay on disk. `wow_status` warns when the current
    dump is such a dump. Domain predicate: `snapshot.IsUnloadedDump`.
  - `wow_trainer`: a window without an NPC name no longer creates an empty entry in `known`, and the response has `npcNote` (assumption: this is the pet's Beast Training window,
    the cost in training points). `wow_shopping_list` and the hints do not count it as a purchase from a trainer.
  - `wow_pet`: the Beast Training window saved as a trainer window without an NPC (as in Forever) is used for rank assessments and the training plan (`windowNote`).
  - `wow_flights`, `wow_quest_route`: abbreviated zone names in flight point names (`Lakeshire, Redridge` → Redridge Mountains,
    `Stormwind, Elwynn` → Stormwind City) and skipping retired `zzOLD…` entries; `withoutNote` explains that the flights window shows
    usually one continent.
  - A recipe ingredient without a name gets the name "item <id>" (`snapshot.ReagentName`).
- **Changed**
  - Version 0.9.1; `wow_status` expects addon 0.7.1.
  - `wow_social`: "the character is not in a guild" instead of a hint about the guild window; it distinguishes "the friends window has not been
    read yet" from "the friends list is empty"; for a guild without a member list it suggests opening the guild window.
  - The `real` golden scenario and `fixtures.Real` use the dump from addon 0.7.0
    (`testdata/real/snapshot_2026-11-15_addon070.json`); the old 0.5.0 dump was removed. `fixtures.RealUnloaded` is the real
    "empty" dump from exiting the game, and `fixtures.RealAt` is a clock relative to the real dump.
- **Added**
  - `snapshot.UnloadedSignals`, `storage.History.SkippedUnloaded`, `progress.Tracker.NoteSkipped`.

## Addon 0.7.0, server 0.9.0 (2026-11-15)

The addon saves more data (XP by source, buyback list, equipment sets, friends, guild, PvP, aura timers, flight point
zone) and gets a minimap button and a compact panel mode. The server reads the new sections, has two new tools
(`wow_quest_route`, `wow_social`) and a lighter history. **Neither addon 0.7.0 nor server 0.9.0 had been run
in a real client.** The API names for the new sections are candidates that must be confirmed with `/mcp probe` (the scope of
`probe` now also covers `C_EquipmentSet`, `C_FriendList`, `C_GuildInfo`, `C_PvP`, `C_Map`). The principle is unchanged: the addon reads
only what the client already has in memory and **never asks the game server on its own** for a friends or guild list (no
`ShowFriends`, `GuildRoster`, `SendWho`); the addon test verifies that none of these calls is made.

### Addon 0.7.0

- **Added**
  - `auras[].expiresAt` (Unix time of the end) and `duration` (duration in seconds); an aura without a duration has neither field.
  - `flightPaths.nodes[].zone`: the flight point's zone from `C_Map.GetMapInfoAtPosition` (or from a client field, if present);
    an entry without a position gets no zone.
  - `xpGains`: your own XP since the previous write to disk (like `loot`): `since`, `total`, `lastAt` and `kills`, `quests`
    and `other`, each `{count, xp}`. Quest XP is counted once, despite the chat message and the quest turn-in event.
  - `buyback`: the buyback list from the vendor window (`npc`, `capturedAt`, `items` with `index`, `name`, `id`, `count`,
    `priceCopper` for the whole batch).
  - `equipmentSets`: equipment sets (`name`, `id`, `isEquipped`, `missing`, `items` with `slot`, `id`, `name`), from `C_EquipmentSet`
    or classic functions; up to 30 sets.
  - `friends` and `guild`: lists from client memory after opening the social window and the guild window (`FRIENDLIST_UPDATE`,
    `GUILD_ROSTER_UPDATE`, `PLAYER_GUILD_UPDATE`). Friends up to 100; guild up to 500 members, and with more than 200 people only
    those online (`onlineOnly`).
  - `pvp`: honorable and dishonorable kills, highest rank, honor, honor level, rank and its name; every field optional.
  - Minimap button (`MinimapButton.lua`): a left click shows or hides the panel, **never saves** and does not reload;
    it can be dragged. `/mcp minimap on|off`.
  - Compact panel mode: a single line with the dump age and the Save button. `/mcp compact on|off`.
- **Changed**
  - `/mcp probe` covers the new namespaces and function prefixes (equipment sets, friends, guild, PvP, buyback, map, XP).
  - `/mcp help` describes the new windows and commands; `WowMcp.toc` has new section files (`EquipmentSets`, `Pvp`, `Buyback`,
    `Friends`, `Guild`, `XpGains`) and `MinimapButton`.
  - The mocks in `addon/tests/run_addon.py` cover the new API; the mock clock is simple and identical for every dump.
    Addon version: 0.7.0.

### Server 0.9.0

- **Added**
  - `wow_quest_route` (`character`): a suggested order of zones with quests (current zone, then by the number of quests ready
    to turn in and of all quests, red last) with known flight points. This is not the shortest route: the server has no world
    map, and the `assumption` field says so. Package `game/questroute`.
  - `wow_social` (`character`, `includeOffline`): friends and guild (online, level, class, zone, rank, counts, age of
    the record). When a list is missing, the response says which window to open in the game and that `/mcp reload` is needed afterwards; it reminds that the addon
    does not ask the game server for these lists on its own. Package `game/social`.
  - `xpSources` (XP from kills, quests and other) in `wow_changes` (with `partialNote`, like loot), in `wow_progress` (`session`,
    `overall`) and in `wow_journal` (session and Markdown); summed from the session's dumps without the first. No `xpGains` section: the field is omitted.
    Package `game/xpgains`.
  - `buyback` in `wow_vendor`: what can still be bought back, the price (per batch), `canAfford` and the age of the record. The response marks it as an
    **assumption** that the client loses the buyback list after logout. No alert was added to `wow_status`: buyback entries have no item
    quality, so it is not possible to reliably judge what is worth buying back.
  - `equipmentSets` in `wow_equipment` and `pvp` in `wow_character`; the sections `xpGains`, `buyback`, `friends`, `guild`, `equipmentSets`
    and `pvp` in the table of windows and tools (`wow_status`, `howToRefresh`, the `first_run` prompt) and in the addon update note.
  - History: `--history-max-mb` (default 200, 0 = no limit; the oldest dumps disappear first, the newest always stays)
    and `--history-compress-days` (default 7, a negative value turns it off): dumps older than that many days are gzipped
    (`<dumpedAt>.json.gz`), reading supports both formats, and an interrupted compression does not lose a dump.
- **Changed**
  - `flights.MatchZone`, `flights.ParseNodes` and the types `Node`, `Match` are exported; `wow_quest_route` uses the same
    matching as `wow_flights` (no duplicated logic, behavior unchanged).
  - `wow_vendor`: the `query` argument also narrows the buyback list.
  - Tool descriptions and server instructions mention the new tools and fields; golden tests and the contract test cover the new tools
    and arguments (48 tools, 14 prompts).
  - The `game/addon` and `mcpserver` tests read the expected dump time from the fixture instead of a hard-coded number.
  - Server version: 0.9.0; expected addon version: 0.7.0.

## Server 0.8.0 (2026-11-15)

Eight new read-only tools, the `tydzien_w_grze` prompt (now `week_in_game`) and MCP resources. The addon is unchanged (0.6.0). Version 0.8.0
**had not yet been run in a real client**: the tools were tested on mocks, the real dump from 0.5.0
and golden tests.

- **Added**
  - `wow_timeline` (`character`, `limit`, `kind`, `days`): a character timeline from the differences between consecutive history dumps:
    level-ups, new abilities, gear changes, quests turned in, deaths and profession thresholds 75/150/225/300. An event gets
    the time of the first dump in which it is visible (the game saves data only on `/reload` and logout); deaths have the exact time.
  - `wow_buffs` (`character`): active buffs and debuffs and missing standing class buffs (heuristic: only when the character knows
    the spell and has no effect from its group among the auras). Aura durations are not in the dump yet, so the time remaining
    appears only when the dump has `expiresAt` or `remaining` (from an addon that does not exist yet); the response says so in `timingNote`.
  - `wow_flights` (`character`): known flight points matched against the zones of quests from the log. `flightPaths` entries have no
    zone, so matching is by the point's name (`matchedBy`); the meaning of the `unreachable` state is unconfirmed
    (an assumption to check in the game, as the response says).
  - `wow_session_rating` (`character`): the rate of the last session (XP, gold balance, quests turned in per hour) against the median of
    earlier sessions; at least three sessions of 2 dumps and 5 minutes of play are needed.
  - `wow_compare_characters` (`a`, `b`): two characters of the account side by side (level, gold, play time, professions, average ilvl,
    gear slot by slot) with a warning when the dumps come from different moments.
  - `wow_alt_mail` (`from`, `to`, `query`): what to mail between characters. Strong evidence: the material is an ingredient of
    a saved recipe of the recipient. When the recipient has no saved recipes for the given profession, a weak lead remains from the kind of
    material (`evidence=profession`, `weakEvidence`, the `assumption` text); the table "profession → Trade Goods subtype" has only
    the obvious pairs from Classic. It skips bound and quest items; it sends nothing.
  - `wow_account_crafting` (`character`, `profession`, `query`, `craftable`, `limit`): for every saved recipe, how many
    can be made from the crafter's materials and the whole account's, who holds what, what is missing and where to send it from.
  - `wow_make_or_buy` (`character`, `profession`, `query`, `sort`, `limit`): ingredient cost (the auction price or the vendor's,
    whichever is cheaper) versus the price of the finished item; verdict `make`, `buy` or `unknown`, the margin and the margin after the 5%
    commission. It assumes one craft yields one item, and never treats a missing price as zero (then `unknown` and `missingPrices`).
  - The `tydzien_w_grze` prompt (`week_in_game`): a summary of the last 7 days from `wow_gold_history`, `wow_journal` and `wow_timeline`.
  - MCP resources: `wow://index` (characters and section addresses) and the template `wow://<character>/<section>` (JSON of the same section as
    `wow_section`). Registered once at startup, with no change notifications.
  - Packages `game/timeline`, `buffs`, `flights`, `sessionrating`, `compare`, `altmail`, `accountcraft`, `makeorbuy`.
- **Changed**
  - `items.BagBankStock`: a shared counter of items from bags and bank (used by `altmail` and `accountcraft`).
  - `auctions.AuctionCut` is exported and also used by `makeorbuy` (one commission rate).
  - Server instructions mention the new tools and prompt; golden tests and the contract test cover every new tool and argument.
  - Server version: 0.8.0.

### Project tooling

- **Added**
  - Addon lint: `.luacheckrc` (Lua 5.1, the list of WoW globals used by the addon). `luacheck addon` gives 0 warnings
    in 42 files; the only fix was a removed unused local in `Sections/Deaths.lua` (test dumps byte for byte unchanged).
  - `.github/workflows/ci.yml`: `go vet`, `go test`, three addon dumps compared with `server/testdata` and `luacheck`.
    The file awaits the day the repository is published; it has not been run on GitHub.
  - an `install.ps1` option (off by default): adds the `wow` server to the client configuration file
    (also in the Microsoft Store version of the app), only when the entry does not exist yet, with a dated copy of the file. It does not create or
    overwrite a missing or damaged file. Verified in PowerShell 7 on temporary files, not on Windows.
  - `docs/FORMAT_ZRZUTU.md` (now `docs/DUMP_FORMAT.md`): a description of the dump format (sections, fields, types, when collected, since which addon version).
  - `docs/PIERWSZE_URUCHOMIENIE.md` (now `docs/FIRST_RUN.md`): a "symptom, cause, what to do" table and rolling back to the previous version.

## Addon 0.6.0, server 0.7.0 (2026-11-15)

The first version fixed on the basis of a real dump from the game (addon 0.5.0 and server 0.6.0 worked in the Forever
1.60.1 client with no section errors; the dump took 18 ms).

### Addon 0.6.0

- **Added**
  - Quest rewards in the log: `quests[].rewards` with `choices` (items to choose from), `items` (fixed), `money`, `xp`
    and `currencies`; items have the same details as in bags, and gear also the tooltip. The new API reads rewards by quest
    ID, without selecting it in the log; the old API selects the entry only for the duration of the read and immediately restores the previous
    selection (when the client does not allow reading it, the rewards are skipped).
  - The sections `questOffer` (a quest offered by an NPC, the `QUEST_DETAIL` event: title, objectives, rewards, NPC) and `questTurnIn`
    (choice rewards when turning in a quest, `QUEST_COMPLETE`). Only the last window of each kind is kept.
  - The `loot` section: the player's own loot since the previous dump written to disk (`since`, `items` with name, ID, quality,
    total count and `lastAt`; up to 100 distinct items). It recognizes only messages about your own loot (`LOOT_ITEM_SELF`
    and related, and without them the English "You receive…"); other players' loot is skipped. After login and `/reload` it starts from zero.
  - `vendors` and `trainers`: up to the 10 most recent vendors and trainers (NPC name → the same as `vendor` / `trainer`);
    `vendor` and `trainer` remain as the last one visited.
  - The `stable` section: stabled pets (slot, name, level, family) after opening the stable window.
  - The auction house price in the item tooltip: an "AH: <price> (<age>)" line when the addon knows the price; `/mcp ahtip on|off`
    (on by default). It is only text in the tooltip.
  - `talents.missingDescriptions`: how many talents still have no description (then it is worth dumping once more).
  - `/mcp probe` also shows the quest reward functions, `C_QuestInfoSystem` and `TooltipDataProcessor`.
- **Changed**
  - `talents.nodes[].row` and `column` are the row and column numbers in the tab grid (from 1, row 1 is the top of the tree) instead of
    raw pixel positions from `C_Traits`; the raw values are in `posX` / `posY`. The grid is computed from the data
    (clusters of positions and the step between them, shared by all tabs and both axes), jitter (5740 instead of 5730)
    and positions 10 times too large (39300, 102800) are corrected. On the real hunter dump every tab has rows 1–7
    and columns 1–4.
  - Addon version 0.6.0.
- **Fixed**
  - Missing descriptions of some rank 0 talents in the real client (`C_Spell.GetSpellDescription` returns an empty string until the
    client loads the spell data): the addon then takes the text from the spell tooltip (`descriptionSource = "tooltip"`)
    and asks the client to load the data (`C_Spell.RequestLoadSpellData`), so the next dump has the description.

### Server 0.7.0

- **Added**
  - A real dump from the game (addon 0.5.0, Forever 1.60.1) as test data: `testdata/real/snapshot_2026-11-15.json`,
    the helper `fixtures.Real` / `fixtures.RealLua` (turns it into a SavedVariables-style file at test time) and the
    `real` scenario in golden tests (all tools and prompts with default arguments and cases only for it).
  - `wow_goals` and `wow_goal_set`: the player's goals (`level`, `gold`, `reputation`, `profession`, `spell`, `item`) with a note
    and a character. `wow_goals` shows the current state, how much is missing, percent and an ETA in play hours and sessions (rate from the history,
    the same as in `wow_progress`). `wow_goal_set` is **the only tool that writes**, and only to the server's goals file
    (`goals.json` next to the history directory, the `--goals-file` flag, the same game-directory guard as `--log`); an atomic write
    (temporary file and rename), a damaged file gives a readable error and is not overwritten.
  - `wow_journal`: a session journal from the history (start, end, duration, levels, XP, gold change, quests turned in and new,
    notable items, deaths, zones) and records (the longest session, the best by gold, the fastest level
    from `/played`). `markdown=true` returns ready text in the `text` field; the server itself does not write `.md` files.
  - `wow_bag_plan`: what to do with full bags (`keep`, `bank`, `sell`, `auction`) with a reason for each item
    and totals (slots freed, vendor value, auction profit). It uses quest ratings, recipes, auction prices,
    the pet and gear comparisons; it never advises destroying items.
  - `wow_reagents`: stocks of ammo, food and water, bandages, potions, other consumables and reagents, consumption per
    play hour from the history and how many hours they last (`lowStock`); pet food separately.
  - `wow_compare_item`: optional `weights` (stat → weight) or a `weights.json` file next to the goals file (weights for a
    character or class); every comparison gets `weightedDelta`. The server has no built-in class builds.
  - Talents: `posX` and `posY` (raw position from the dump) and `missingDescriptions` with a note that descriptions appear after
    opening the talents window and the next `/mcp reload`.
  - The `game/consumption` package (consumption per hour from the history, shared by `wow_ammo`, `wow_reagents`), `progress.Rates`,
    `progress.Tracker.Sessions` and `storage.OwnFile` (the server's own file, atomic write).
  - The contract test (`contract_test.go`): every dump section read by name in domain code must exist in some
    test dump, and every tool with default arguments must work on every dump.
  - `wow_quest_rewards` (arguments `character`, `quest` name or ID, `weights`): an evaluation of choice rewards for quests
    against the equipped gear. At the top the quest being turned in and the one offered by an NPC (`questTurnIn`, `questOffer`, only when the
    window is fresh), then quests from the log (`quests[].rewards`). Each reward has a slot, stat differences, a score
    (from weights: the `weights` argument or `weights.json`; without them item level and DPS, and the response says which method was used),
    vendor value and a `usable` flag; `best` points to a choice with a justification (an upgrade, and when there is none, the highest price
    at the vendor; unusable rewards never win over an upgrade). Fixed rewards, gold and XP separately. **Safeguard:**
    the addon reads rewards by quest ID, and the client could return the selected quest's rewards for all of them; when 2 or more
    quests from the log have an identical non-empty list of choice rewards (the same IDs in the same order), they get `suspect: true`,
    a "verify in the game" note and **no recommendation**. New package `game/questrewards`, `items.Evaluate` and `items.ChooseWeights`.
  - `wow_shopping_list` (arguments `character`, `hours` default 2): what to buy before leaving town. Trainer (`available`
    services from all remembered trainers, the newest offer of each NPC, the same ability at two trainers counts
    once at the lower price; flags `stale` for offers older than `--stale-minutes` and `maybeOutdated` when a service saved as
    unavailable has a required level not higher than the current one), repair, ammo (how much to buy for `hours` hours), pet
    food, running-low materials from `wow_reagents`, and for each item WHO sells it and for how much (vendors from `vendors`,
    price of the pack and the item). `budget` checks in priority order (trainer, repair, ammo, food) what the gold covers.
    New package `game/shopping`.
  - `returnToTown` (in `wow_shopping_list` and in the `plan_sesji` prompt data (`session_plan`)): when to return to town. The nearest level at which
    something new appears at the remembered trainers (from services with a required level higher than the current one), in how many play hours the
    bags fill up (growth of occupied slots from the history; `consumption.MeasureGain`), how many hours the ammo lasts (`ammo.Left`)
    and durability. When there is too little history, the field says "no data".
  - `wow_pet` additionally shows: `rankReview` (ranks known to the Hunter from the Beast Training window that the pet does not have, and ranks
    from trainer offers that the Hunter does not know yet; only from dump data, no built-in tables), `trainingPlan` (which of the trainings
    fit in the free training points; the order is an explicit heuristic: higher ranks of known abilities first, from
    the cheapest) and `stable` (stabled pets compared with the active one by level and family). The same data goes to the `zwierzak` prompt (`pet_care`).
  - Loot: `wow_journal` shows `loot` for each session (counts and notable loot by quality, also in Markdown), and `wow_changes` the
    `loot` field from the newer dump. New package `game/loot`.
  - `wow_vendor` and `wow_trainer` accept `npc` (the name or a fragment of one of the remembered NPCs) and return `known`
    (remembered NPCs). `snapshot.Visits` joins the sections `vendor` / `vendors` and `trainer` / `trainers` (the newest offer of each NPC).
  - `wow_search` also searches `vendors`, `trainers`, `questOffer`, `questTurnIn`, `loot`, `stable` and quest rewards;
    the path of a hit in an offer carries the NPC name.
  - `wow_status` (`howToRefresh`) and `first_run` (`windows`) mention the stable window and the quest windows at NPCs.
- **Changed**
  - Talents: `row` and `column` from addon 0.5.0 (pixels with deviations, e.g. 5740, 39300, 102800) are converted on the
    server side to row and column numbers from 1 within the tree (a deviation of exactly 10x gets the number of the
    base value); the raw values stay in `posY` and `posX`.
  - `wow_talents`: `canBuyNow` is empty when there are no free points (`unspent` = 0); the game's `canPurchase` flag does not
    account for free points.
  - `wow_ammo` computes consumption through the shared `consumption` package (response unchanged).
  - The writing tool has no "read-only" marker; server instructions mention the new tools, and `wow_status`
    shows `goalsFile`.
  - The tests `TestVersionNote`, `TestTimingsAndDisabled` and `TestServerEndToEnd` take the expected version from `addon.ExpectedVersion`
    instead of a hard-coded number, so bumping the addon version does not break them.
  - Search: `npc` is also the label of a record without a title and name.
  - `snapshot.Freshness.IsStale` (whether the moment of the write is older than the threshold).
- **Fixed** (found on the real dump)
  - Item comparisons counted DPS twice: the `damage per second` key in `stats` is now DPS, and not a separate
    stat next to the DPS from the description.
  - `pet`: stances and commands (`Pet Stance`, `Pet Command`, huge IDs) are no longer listed as pet abilities.
  - An ETA to level-up under a minute showed "0 min"; now "under 1 min".
  - Server version: 0.7.0.

## Addon 0.5.0, server 0.6.0 (2026-11-15)

### Added

Addon:

- The `deaths` section: a death counter and up to 20 recent ones (zone, subzone, level, time) from the `PLAYER_DEAD` event.
- The `/mcp note <text>` command: a note for the assistant in the dump (up to 500 characters); `/mcp note` alone removes it.
- The `/mcp off <section>` and `/mcp on <section>` commands: turning off collection of a section (except `character`); `/mcp off` alone shows the names.
- Dump timing (`timings`): `/mcp status` gives the total time and the slowest sections, and warns above 200 ms.
- `character.resting` (whether the character is in an inn or a city) and `character.bindLocation` (where the Hearthstone returns to).
- Panel: the hover tooltip shows section errors; the "Save" button is highlighted when a window (trainer,
  vendor, auctions and others) has produced new data. The panel saves nothing on its own.
- `probe.events`: which events the client accepted in `RegisterEvent`.
- `/mcp status` shows the addon version, dump time and disabled sections.

Server:

- Tools: `wow_search` (text search across the whole dump), `wow_rested` (rested XP and when it will be full),
  `wow_gold_history` (gold day by day from the history), `wow_leftovers` (quest leftovers in bags and bank),
  `wow_ammo` (ammo stock, consumption per hour, how much to buy).
- Prompts: `przed_wylogowaniem` (`before_logout`), `pierwszy_raz` (`first_run`), `zwierzak` (`pet_care`).
- `wow_status`: `serverVersion`, `expectedAddonVersion`, `addonVersion` for the character, dump time (`dumpTimeMs`), disabled sections,
  the player's note and `alerts` (expiring mail and auctions, damaged gear, full bags, slow dump, addon errors).
- `wow_character` shows `note` and `deaths`; `wow_changes` and `wow_progress` show new deaths.
- Read retries (up to 4 attempts in about 600 ms) when the game is writing the file at that moment.
- The `--log <file>` flag: errors and startup information to a file. History and log paths inside the game directory are rejected.

Project:

- `install.ps1`: deploying the addon and server with one command; `-Rollback`, `-KeepOld`, `-Build`, `-WhatIf`, `-SkipCheck`,
  the Interface number check.
- `docs/PIERWSZE_URUCHOMIENIE.md` (now `docs/FIRST_RUN.md`), `docs/JAK_PYTAC.md` (now `docs/HOW_TO_ASK.md`), `docs/ZGODNOSC_Z_REGULAMINEM.md` (now `docs/TOS_COMPLIANCE.md`), this file.
- A local git repository.
- Golden response tests of the server through the MCP protocol and parser fuzzing.

### Changed

- `character.hearthstone` replaced by `character.bindLocation`.
- `wow_export`: the warning about text length (over 2000 characters for Discord) is in the `lengthNote` field; `warning` remains for
  stale data. In the Discord format the title has a `###` heading, and the rest of the text is no longer modified.
- The server expects addon 0.5.0 (`ExpectedVersion`); an older addon gets an update request in `wow_status`.
- `timings` and `disabledSections` are diagnostic fields, not character state (they do not enter comparisons).

### Fixed

Addon:

- The `condition and a or b` pattern returned a wrong value when `a` was `nil` (auctions, gear, bags, recipes); now `Compat.pick`.
- Browsing the auction house: several keys with the same name overwrote each other; now the lowest price and the sum of counts.
- Bank: the order of bags depended on `pairs()`; now it is ascending, and the dump is reproducible.
- API values without `plain()` (so-called secrets in combat): map position, client version, character data, quests, flights, talents.
- The LFG activity list: the limit applied to one category instead of the whole list.
- Gear: a read error on one slot broke the others.
- Pet: `autoCast` confused "the ability does not support this" (`nil`) with "off" (`false`).
- Flight map: a node type that is not a string ended in an error.

Server:

- Parser: deeply nested tables overflowed the stack (limit of 200 levels); block comments were processed in quadratic time.
- A dump that after loading is not a non-empty object (`{}`, `null`) gives an error instead of a panic.
- `wow_changes`: `completedCountChange` only when both dumps have a list of completed quests.
- `wow_progress`: time to level-up computed without the helper field in the response.
- `wow_bags`: slot totals also work on floating-point numbers (e.g. from the history).

### Internal

- Addon: the single `WowMcp.lua` file (2213 lines) split into modules: `Core`, `Compat`, `Items`, `Store`, `Events`,
  `Timing`, `Snapshot` (section registry), `Sections/*`, `Panel`, `Commands`. The three test dumps after the split were byte for byte
  identical. A new section is a new file, a registry call and a line in `.toc`.
- Server: the flat `package main` split into layers: `savedvars`, `snapshot`, `storage`, `game/<context>`, `mcpserver`,
  `fixtures`. `game/*` does not depend on MCP or on the file system. The clock and the "stale data" threshold enter through `snapshot.Freshness`,
  the history through the `progress.History` port. One registry of tools and prompts in `mcpserver`.
- `golden_test.go` calls every tool and prompt through the MCP protocol and compares with `testdata/golden/`; a missing case for
  a tool, prompt or argument is a test error. Refreshing: `go test -run Golden -update .`.
- Parser fuzzing: `go test -run=^$ -fuzz=FuzzParse -fuzztime=75s ./internal/savedvars`.
- The new server is delivered as `bin\wow-mcp.new.exe`; `install.ps1` swaps it to `bin\wow-mcp.exe`
  (a running exe cannot be overwritten, so the old one is renamed to `.old.exe`).

## Earlier versions (addon 0.2.0 to 0.4.0, server 0.1.0 to 0.5.0)

The split into versions was not preserved: the project had no repository then, and the initial state (commit `e22f08e`) is addon 0.4.0
and server 0.5.0. Below is what existed in that state. We give neither dates nor a split into versions.

- Addon: a dump of the character, statistics, buffs, quest log, completed quests, bags and gear (with stats, descriptions
  and ammo), abilities, talents, professions, pet, action bars, reputation, macros, currencies, cooldowns, play time,
  group and dungeons. Sections from windows: trainer, vendor (with repair cost), bank, flight map, recipes (each profession
  separately), mail, auction house (prices from search results and your own auctions), Inspect.
- Addon 0.4.0: full talent trees with descriptions (`trees`, `nodes`, `canPurchase`).
- Addon: a panel with the dump age and a "Save" button, the commands `/mcp reload`, `/mcp status`, `/mcp panel`, `/mcp played`,
  `/mcp probe`, `/mcp help`.
- Server: the tools `wow_status`, `wow_character`, `wow_quests`, `wow_bags`, `wow_equipment`, `wow_spells`, `wow_pet`,
  `wow_trainer`, `wow_reputation`, `wow_bank`, `wow_vendor`, `wow_compare_item`, `wow_changes`, `wow_quest_plan`,
  `wow_progress`, `wow_recipes`, `wow_mail`, `wow_cooldowns`, `wow_account`, `wow_find_item`, `wow_talents`,
  `wow_auctions`, `wow_group`, `wow_dungeons`, `wow_export`, `wow_section`.
- Server: the prompts `plan_sesji` (`session_plan`), `zakupy_u_trenera` (`trainer_shopping`), `co_sprzedac` (`what_to_sell`), `ulepszenia_ekwipunku` (`gear_upgrades`), `podsumowanie_sesji` (`session_summary`),
  `przeglad_konta` (`account_overview`), `rozwoj_profesji` (`profession_progress`), `rozwoj_talentow` (`talent_planning`), `co_wystawic_na_aukcji` (`what_to_auction`), `gdzie_na_dungeon` (`dungeon_finder`).
- Server: dump history (`--history-dir`, `--history-keep`), a warning about stale data (`--stale-minutes`), `--check`,
  a request to update the addon when the version is older.
