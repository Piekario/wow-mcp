# The `WowMcp.lua` dump format

A description of the SavedVariables file that the WowMcp addon writes and the wow-mcp server reads. As of addon 0.10.0 (schema 2).
Sources: `addon/WowMcp/Sections/*.lua`, `Snapshot.lua`, `Store.lua`, `CHANGELOG.md` and the test dumps in `server/testdata/`.

File location: `<game>\<client>\WTF\Account\<account>\SavedVariables\WowMcp.lua`. The game writes it only on `/reload`,
logout and exit, so the data is only as fresh as the last such moment.

The **Since** column is the addon version in which the field appeared. "≤0.4.0" means: it was already in the repository's initial state
(addon 0.4.0), the exact version is unknown.

## 1. Top-level structure

The file is a single Lua variable, `WowMcpDB`:

```text
WowMcpDB
├── schema      number, schema version (currently 2)
├── last        string, key of the character from the last dump on the account
├── settings    addon settings (not character state)
└── characters  map: character key → dump (snapshot)
```

- The character key is `Name-Realm` (when the client provides the realm), otherwise `Name-<realm number>` from the player's GUID (addon 0.20.0, also saved as `character.realmId`), otherwise just the name. Every character on the account has its own dump,
  overwritten on that character's next dump.
- `settings` contains: `disabledSections` (map section name → `true`, since 0.5.0), `panelHidden`, `panelPoint`, `played`
  (`false` after `/mcp played off`) and `ahTooltip` (`false` after `/mcp ahtip off`, since 0.6.0), `minimap` (`false` after `/mcp minimap off`), `minimapAngle` (position of the minimap
  button in degrees) and `compact` (`true` after `/mcp compact on`), all since 0.7.0. Since the sharing window: `shareReminder` (`false` after `/mcp share off`) and `share` (`remindedAt`, and per "class|build|locale": `asked`, how many reminders were printed, and `opened`, when the window was first shown). The server does not read them.
- The server keeps its own history (`history\<character>\<dumpedAt>.json`), so the game file always holds only the latest dump.

## 2. When a section ends up in the dump

| Kind | Meaning |
|---|---|
| **Every dump** | collected afresh on every dump (`/mcp`, `/mcp reload`, logout) |
| **From a window** | saved at the moment a window is opened in the game (an event), then **carried over** from the previous dump until the player opens the window again; its `capturedAt` says how old the data is |
| **Live** | saved by events during play and carried over between dumps |
| **Meta** | a field describing the dump itself |

A section can be turned off (`/mcp off <section>`, except `character`); a disabled one is not collected and disappears from the dump.
An error in one section does not break the others: it goes to `errors`.

## 3. Dump fields (meta)

| Field | Type | Description | Since |
|---|---|---|---|
| `dumpedAt` | number | Unix time of the dump; the server uses it as the key in the history | ≤0.4.0 |
| `dumpedAtText` | string | the same time as `YYYY-MM-DD HH:MM:SS` (the player's local time) | ≤0.4.0 |
| `reason` | string | what triggered the dump: `manual` (the `/mcp` or `/mcp reload` command) or `logout` | ≤0.4.0 |
| `lastSkippedLogoutAt` | number | Unix time of the last exit from the game at which the client had already released the character (health 0, empty spells, equipment, bags without size; at least two of these signals); the addon then kept this dump instead of writing an empty one. When `lastSkippedLogoutAt >= dumpedAt`, the `loot` / `xpGains` period continues after the next login | 0.7.1 |
| `clientUnloaded` | `true` | a dump from exiting the game with the character released, when there was no previous one to keep; the server skips it in the progress history | 0.7.1 |
| `addonVersion` | string | addon version; the server compares it with `ExpectedVersion` | ≤0.4.0 |
| `errors` | list of strings | section errors in the form `section: message`; empty when everything succeeded; the server shows them as `addonErrors`. Since 0.7.1 also event handler errors from the current session as `event NAME: message` (at most 10, no repeats) | ≤0.4.0 |
| `timings` | table | `totalMs` (time of the whole dump) and `slowest` (list of `{name, ms}` of the slowest sections); diagnostics | 0.5.0 |
| `disabledSections` | list of strings | names of sections turned off with `/mcp off`; the field is absent when nothing is disabled | 0.5.0 |

The server treats `timings` and `disabledSections` as diagnostic fields, not character state (they do not enter dump comparisons).

## 4. Sections collected on every dump

### `character`: character card (table)

| Field | Type | Description | Since |
|---|---|---|---|
| `name`, `class`, `classToken`, `race`, `raceToken`, `faction` | strings | character identity | ≤0.4.0 |
| `level`, `xp`, `xpMax` | numbers | level and experience | ≤0.4.0 |
| `moneyCopper` | number | gold in copper | ≤0.4.0 |
| `mapID`, `x`, `y` | numbers | position on the map (`x`, `y` in percent, 1 decimal place); may be empty in combat or an instance | ≤0.4.0 |
| `addonVersion` | string | same as the dump field | ≤0.4.0 |
| `clientVersion`, `clientBuild`, `clientInterface` | string, string, number | game client version | ≤0.4.0 |
| `zone`, `subZone` | strings | zone and subzone | 0.5.0 |
| `bindLocation` | string | where the Hearthstone returns to (replaced `hearthstone`) | 0.5.0 |
| `resting` | boolean | whether the character is in an inn or a city | 0.5.0 |
| `restedXp` | number | rested XP pool | 0.5.0 |
| `questGreenRange` | number | how many levels below which quests are gray (no XP) | 0.5.0 |
| `maxQuests` | number | quest log limit | 0.5.0 |
| `locale` | string | the game client language from `GetLocale()` (`enUS`, `deDE`, `ruRU`...). Absent in older dumps, which the server treats as English. Names, types and tooltip text in the whole dump are in this language | 0.9.0 |

### `stats`: statistics (table of numbers)

`strength`, `agility`, `stamina`, `intellect`, `spirit`, `armor`, `attackPower`, `rangedAttackPower`, `critChance`,
`rangedCritChance`, `dodgeChance`, `parryChance`, `healthMax`, `manaMax`, `meleeDamageMin/Max`, `meleeSpeed`,
`rangedDamageMin/Max`, `rangedSpeed`. Fields the client does not return are omitted. Since ≤0.4.0.

### `auras`: buffs and debuffs (list, up to 40)

`{id, name}` (number, string). Since ≤0.4.0. Additional fields: `harmful` (`true` for a debuff), `stacks` (when more than 1),
`expiresAt` (Unix time the aura ends, converted from `expirationTime` on the `GetTime()` clock; absent when the aura has no duration)
and `duration` (total duration in seconds); the last two since 0.7.0. `icon` (0.10.0, number): the aura's icon file ID
(`AuraData.icon`); with the spell ID `id` it lets the server match an aura to a known spell in any game language.

### `quests`: quest log (list)

| Field | Type | Description | Since |
|---|---|---|---|
| `id`, `title`, `level`, `zone` | number, strings | basic quest data | ≤0.4.0 |
| `zoneMapID` | number | `UiMapID` of the zone named by the quest log header (the addon looks the header up among the zone maps of the world map's continents and of the player's continent, `C_Map.GetMapChildrenInfo`), the same number in every client language; absent for headers that are not zone names and when the client has no map API. The server matches quest zones to flight points by it | 0.10.0 |
| `complete` | boolean | whether the objectives are done | ≤0.4.0 |
| `objectives` | list | `{text, have, need, done}` | ≤0.4.0 |
| `rewards` | table | rewards: `choices` (to choose from), `items` (fixed), `money`, `xp`, `currencies`. Items have the same fields as in `bags` (gear also `stats`, `tooltip`, `usable`) | 0.6.0 |

### `completedQuests`: completed quests

A list of ID numbers (numbers). Since ≤0.4.0.

### `bags`, `equipment`: bags and equipped gear

`bags`: a list of bags `{index, name, size, free, items}`. An item in a bag: `slot`, `id`, `name`, `count`, `quality`, `itemLevel`,
`requiredLevel`, `type`, `subType`, `equipLoc`, `sellPrice` (copper per item), `tooltip` (list of tooltip lines, gear only)
and `stats` (item statistics from `GetItemStats`; the keys are the client's translated labels, e.g. `Stamina`, `Ausdauer`). Since ≤0.4.0.

Locale-independent item fields (0.9.0; absent when the client gives no value):

| Field | Type | Description |
|---|---|---|
| `classID`, `subclassID` | numbers | `Enum.ItemClass` and its subclass enum (returns 12-13 of `GetItemInfo`, or 6-7 of `GetItemInfoInstant`). Also on vendor items, quest rewards, the bank and every other item written through `Items` |
| `bindType` | number | `Enum.ItemBind`: 0 none, 1 on pickup, 2 on equip, 3 on use, 4 quest (return 14 of `GetItemInfo`) |
| `statTokens` | table | the same statistics as `stats` with the game's global token names as keys (`ITEM_MOD_STAMINA_SHORT`, `ITEM_MOD_DAMAGE_PER_SECOND_SHORT`, `RESISTANCE0_NAME`...) |
| `quality` | number | now also filled from item info for items that had none (vendor, quest rewards) |
| `isBound` | boolean | bag and bank items: whether this copy is bound (container item info) |
| `isQuestItem`, `questID` | boolean, number | bag and bank items (`GetContainerItemQuestInfo`): `isQuestItem` only when true, `questID` (> 0) when the item begins that quest |

`type`, `subType`, `name`, `stats` and `tooltip` stay translated client text; the server prefers the fields above and falls back
to the English text only for older dumps.

Weapon numbers (0.10.0; weapons whose tooltip the addon reads: equipment, bags, bank, vendor, quest rewards; absent for other items):

| Field | Type | Description |
|---|---|---|
| `speed` | number | attack speed in seconds (2 decimals). Read from the damage line's right-hand text, or after the client's `WEAPON_SPEED` label; the client's text is recognised by its own global format strings, so it works in every language and with a decimal comma (`Tempo 1,60`) |
| `damageMin`, `damageMax` | numbers | the damage range, found with the client's `DAMAGE_TEMPLATE` (`%s - %s Damage`) |
| `dps` | number | `ITEM_MOD_DAMAGE_PER_SECOND_SHORT` from `statTokens` when present, else the `DPS_TEMPLATE` line, else (min+max)/2/speed; 1 decimal |

The client has no numeric weapon speed in `C_Item` (checked against `/mcp probe`), so the numbers come from the tooltip lines.

`equipment`: a list of items with `slot` (string, e.g. `MainHandSlot`) and `durability` and `durabilityMax`; the remaining fields as in bags,
including ammo. Since ≤0.4.0.

### `spells`: spellbook (list)

`{id, name, rank, tab}`. Since ≤0.4.0. Since 0.10.0 `icon` (number, the spell's icon file ID from `iconID` of the spellbook entry; every rank of
a spell shares it) and `selfBuff` (`true` when `C_Spell.IsSelfBuff(id)` says the spell is a buff cast on oneself; only for non-passive spells).
`game/buffs` uses the IDs and icons to match known spells to active auras without names. Pet entries that are commands or stances (`Enum.SpellBookItemType.PetAction`) have `petAction = true` (0.9.0).

### `talents`: talents (table)

| Field | Type | Description | Since |
|---|---|---|---|
| `source` | string | `traits` (C_Traits) or `classic` (classic API) | ≤0.4.0 |
| `spent`, `unspent` | numbers | spent and free points | ≤0.4.0 |
| `trees` | list | `{name, points}` of the tabs | 0.4.0 |
| `points` | list | `{tree, name, rank, maxRank}` only talents with points (classic API) | ≤0.4.0 |
| `nodes` | list | the whole tree, including rank 0: `tree`, `name`, `rank`, `maxRank`, `spellID`, `description`, `choices`, `canPurchase`, `available` (from C_Traits) or `prereqMet` (classic API) | 0.4.0 |
| `nodes[].row`, `column` | numbers | row and column number in the tab grid, from 1 (row 1 = top); up to 0.5.0 these were raw pixels | 0.6.0 (meaning changed) |
| `nodes[].posX`, `posY` | numbers | raw position from `C_Traits` (pixels with jitter, sometimes 10x too large) | 0.6.0 |
| `nodes[].descriptionSource` | string | `tooltip` when the description was taken from the spell tooltip instead of the API; field absent = API | 0.6.0 |
| `missingDescriptions` | number | how many talents still have no description | 0.6.0 |

Descriptions are missing until the client loads the spell data: they appear after opening the talents window and the next dump.

### `legacy`: Forever Legacy system (table, since 0.15.0)

The client documents no API for Legacy, so the addon reads every `C_Traits` config that is not the class talents and also records
what the client calls its Legacy API. Nothing in this section is ever written back to the game.

| Field | Type | Description | Since |
|---|---|---|---|
| `source` | string | `traits` when at least one config was read, otherwise `none` | 0.15.0 |
| `configs` | list | one entry per non-class trait config (`C_Traits.GetConfigsByType` for each `Enum.TraitConfigType` except `Invalid` and `Combat`, plus a `configID` field found on a frame named `*Legacy*`): `configID`, `type`, `typeName`, `name`, `trees`, `nodes`, `currencies`, `missingDescriptions` | 0.15.0 |
| `configs[].trees` | list | `{name, treeID, points}` per tab (node group with a display name; `tree <id>` when the tree has no named groups, as in the real client: 1187 Professions, 1188 Adventure, 1189 Resourcefulness); for an unnamed tree also `nodes` (perk count) and `info` (plain fields of `C_Traits.GetTreeInfo`, 0.17.0) | 0.15.0 |
| `configs[].nodes` | list | like `talents.nodes` (`tree`, `name`, `rank`, `maxRank`, `spellID`, `description`, `descriptionSource` (`trait` = `C_Traits.GetTraitDescription`, or `tooltip`; since 0.17.0 a tooltip with one line under the name is not a description), `canPurchase`, `available`, `row`, `column`, `posX`, `posY`) plus `nodeID`, `prereqMet` (`meetsEdgeRequirements`), `requires` (names of the nodes whose arrow points here), `locked` and `requiresSpent` (an unmet node condition and its `spentAmountRequired`) | 0.15.0 |
| `configs[].currencies` | list | `C_Traits.GetTreeCurrencyInfo`: `currencyID`, `treeID`, `available`, `spent`, `max`; `maxAvailable` (`C_Traits.GetMaxAvailableTraitCurrency`); when the trait currency links an account currency: `currencyTypesID`, `name`, `quantity`, `totalEarned`, `maxQuantity` | 0.15.0 |
| `configErrors` | list | configs that failed to read (`id: error`) | 0.15.0 |
| `challenges` | table | `matchedBy`, `list` of achievements in categories whose title contains "Legacy" (and their subcategories): `id`, `category`, `name`, `points`, `completed`, `earnedByMe`, `description`, `reward`; up to 150. Absent when the client has no achievement API; `challengesError` when reading failed | 0.15.0 |
| `discovery` | table | Since 0.19.1 only when no perk tree was found or after `/mcp probe` (the scan of `_G` cost 70 to 140 ms on the first dump of a session). `enums` (`Enum.X.Y=value` lines for names with "Legacy", and all of `TraitConfigType` / `TraitSystemID`), `namespaces` (`C_*Legacy*` and, since 0.17.0, `C_*Challenge*` name → function names), `members` (0.17.0: `C_X.Name` of functions named for Legacy or challenges in any `C_*` namespace), `readings` (`C_X.GetY` → up to 3 return values of argument-free `Get*`/`Is*`/`Has*`/`Are*` functions in those namespaces and `members`, copied 3 levels deep, 40 entries per table), `globals` (global function names with "Legacy"; never called), `frames` (`name`, `shown`, plain `fields` and, since 0.17.0, `children` (names of table fields) of frames with "Legacy" in the name). Names are scanned once per session and after an addon named for Legacy loads (0.17.0) | 0.15.0 |

### `professions`: professions and skills (list)

`{name, rank, maxRank, skillID, group}`. Since ≤0.4.0. Since 0.9.0 `skillID` (the skill line ID, e.g. 164 Blacksmithing, 393 Skinning)
is also set for professions: from the 7th return of `GetProfessionInfo`, and for the old Skills window by matching the row name against the
client's own name of each known profession skill line (`C_SkillInfo.GetSkillLineInfoByID`). The server tells professions apart by `skillID`.

### `pet`: pet (table, mainly Hunter)

`exists`, `name`, `family`, `level`, `xp`, `xpMax`, `happiness`, `loyalty`, `loyaltyRate`, `dead`, `healthMax`, `damagePercent`,
`foodTypes` (list of strings), `trainingPointsTotal/Spent/Free`, `abilities` (`{id, name, rank}`) and `actionBar`
(`{slot, id, name, autoCast}`). Since ≤0.4.0.

### `actionBars`, `macros`, `reputation`, `currencies`

| Section | Fields | Since |
|---|---|---|
| `actionBars` | list `{bar, button, slot, kind, id, name, key}` (`kind`: `spell`, `item`, `macro`) | ≤0.4.0 |
| `macros` | list `{name, scope, body}` | ≤0.4.0 |
| `reputation` | list `{name, standing, value, floor, ceiling}` | ≤0.4.0 |
| `currencies` | list `{id, name, quantity, max, group}` | ≤0.4.0 |

### `equipmentSets`, `pvp` (since 0.7.0)

| Section | Fields |
|---|---|
| `equipmentSets` | list `{name, id, isEquipped, missing, items}` from `C_EquipmentSet` (in older clients the classic `GetEquipmentSetInfo`); `items` is `{slot, id, name}` without empty and ignored slots; up to 30 sets. No API: the section is absent |
| `pvp` | `honorableKills`, `dishonorableKills`, `highestRank` (`GetPVPLifetimeStats`), `honor`, `honorMax`, `honorLevel` (`UnitHonor*`), `rank`, `rankName` (`UnitPVPRank`, `GetPVPRankInfo`). Every field is optional; with none of them the section is absent |

### `cooldowns`, `played`, `group`, `dungeons`

| Section | Fields | Since |
|---|---|---|
| `cooldowns` | list `{kind, id, name, readyAt, remainingSeconds}`; `readyAt` is Unix time | ≤0.4.0 |
| `played` | `{totalSeconds, levelSeconds}` from `TIME_PLAYED_MSG` (time added since the client's reply); absent when the client has not replied yet; turned off with `/mcp played off` | ≤0.4.0 |
| `group` | composition at the moment of the dump: `type`, `size`, `lootMethod` (name, e.g. `group`; since 0.7.1 also from the number `C_PartyInfo.GetLootMethod`), `members` (`name`, `class`, `classToken`, `race`, `level`, `role`, `subgroup`, `zone`, `online`, `inRange`, `leader`, `masterLooter`, `raidRank`, `healthMax`, `powerMax`) | ≤0.4.0 |
| `dungeons` | `activities` (C_LFGList: `id`, `name`, `category`, level ranges, `maxPlayers`), `lfg` (classic API: `minLevel`, `maxLevel`, `recLevel`...), `lockouts` (`name`, `difficulty`, `locked`, `resetAt`, `encounters`, `killed`); up to 200 entries. IDs since 0.10.0 (same in every language): `activities[].categoryID` and `groupID` (`groupFinderActivityGroupID`); `lfg[].typeID`, `groupID` and `mapID` (`lfgMapID`, the return 22 of `GetLFGDungeonInfo`, the instance's map ID); `lockouts[].instanceID` (return 14 of `GetSavedInstanceInfo`, the instance's map ID, equal to `lfg[].mapID`) and `difficultyID` (return 4). `activities[].id` and `lfg[].id` were already the activity and dungeon IDs | ≤0.4.0 |
| `guide` | the leveling guide the player follows in the Guidelime addon (for example TUGs); absent without Guidelime. `addon`, `name` (the current guide), `nextGuide`, `totalSteps`, `doneSteps`, `notForPlayer` (steps for other races or classes, left out), `next` (up to 15 open steps in guide order: `line`, `text` (the guide's own markup, up to 400 bytes), `quests` (`id`, `action` = `accept` / `complete` / `turnin` / `skip`, `objective`), `optional`, `passed` = an open quest step before the furthest done step; since 0.14.1 at most 5 of those are listed so the list reaches the current step), `passedSteps` (0.14.1: how many open steps lie before the furthest done one). A quest step is done by the quest log (accepted, objectives complete, flagged completed); another step when the player ticked or skipped it in Guidelime (`GuidelimeDataChar.progress`) or a later step is done. `textUnavailable` + `registeredGuides` when the addon did not see the guide's text. The guide text is read when the guide addon registers it (`hooksecurefunc` on `Guidelime.registerGuide`); nothing is changed in Guidelime | 0.14.0 |
| `questDb` | where quests start and end, read from the quest database the player has installed (QuestieDB, global `LibQuestieDB`); absent without it. `source` (`name`, `version`, `contract`, `readMode`), `quests` (up to 60: the quests of the log, then the quests of the `guide.next` steps): `id`, `name`, `level`, `requiredLevel`, `zone`, `zoneAreaID`, `objectivesText`, `nextQuest` (`id`, `name`), `inLog`, `inGuide`, `startedBy` / `finishedBy` / `objectives` (up to 4 targets each; `startedByCount` / `finishedByCount` when there are more). A target: `kind` (`npc`, `object`, `item`), `id`, `name`, `title`, `friendlyTo`, `minLevel`, `maxLevel`, `text` (objective text), `locations` (up to 3: `zone`, `areaID`, `x`, `y` in map percent, or `inInstance`; the target's main zone first), `spawnCount`, and `zones` + `zoneCount` when it spawns in several zones. `unknownQuestIDs`: log quests the database does not know. WowMcp ships no quest data | 0.14.0 |

## 5. Sections from windows (carried over between dumps)

Each has `capturedAt` (Unix time the window was opened) and usually `npc`. The data is true as of the visit, not of the dump.

| Section | Window | Contents | Since |
|---|---|---|---|
| `trainer` | trainer | `npc`, `services` (`name`, `rank`, `category`: `available`, `unavailable`, `used` or `header`; `costCopper`, `levelRequired`) | ≤0.4.0 |
| `vendor` | vendor | `npc`, `items` (`index`, `id`, `name`, `priceCopper` per `stack` items, `stack`, `limitedStock`, `requiredLevel`, `sellPrice`, `stats`, `tooltip`...), `canRepair`, `repairCostCopper` | ≤0.4.0 |
| `trainers`, `vendors` | as above | map NPC name → the same as `trainer` / `vendor`, up to the 10 most recent NPCs; `trainer` and `vendor` remain as the last one visited | 0.6.0 |
| `bank` | bank | `containers` (`index`, `name`, `size`, `free`, `items`), in ascending order | ≤0.4.0 |
| `flightPaths` | flight map | `npc`, `nodes` (`name`, `state`, `zone`, and since 0.10.0 `nodeID` and `zoneMapID`: the client's flight point ID and the `UiMapID` of its zone from the zone map list or the map at its position, absent when the zone came from a node field or is unknown; the server matches quest zones to points by `zoneMapID`). `zone` (since 0.7.0) is the flight point's zone: a node field, the continent zone map (`C_Map.GetMapChildrenInfo` + `C_TaxiMap.GetTaxiNodesForMap`, since 0.7.1) or `C_Map.GetMapInfoAtPosition` at the node's position (skipped when it points to the flight map itself); absent when the client does not provide it. Since 0.12.0 a position that resolves only to a continent (or above) is never a zone: the node gets `continent` (the continent name) instead; addon 0.7.0 wrote the continent as `zone` (e.g. `"Eastern Kingdoms"` on every point), and the server treats a continent name in `zone` as no zone. Diagnostics since 0.7.1: `taxiMapID`, `nodeFields` (fields of the client's raw node as `key:type`), `zoneSources` (zone counts from the sources `node`, `zoneMap`, `position`). The window usually shows the points of one continent; names may include retired `zzOLD…` entries | ≤0.4.0 |
| `recipes` | professions window | map profession name → `{name, rank, maxRank, recipes}`; each profession overwrites only its own entry. Recipe: `name`, `difficulty`, `group`, `itemId`, `canCraft`, `reagents` (`name`, `need`, `have`; from `C_TradeSkillUI` since 0.7.1 also `id`, basic ingredients only), `cooldownSeconds`, `readyAt`; Beast Training: `rank`, `trainingPoints`, `levelRequired`. Forever 1.60.1 has only `C_TradeSkillUI`; it shows the Beast Training window as a trainer window without an NPC (a `trainer` section without `npc`) | ≤0.4.0 |
| `mail` | mailbox | `messages` (`sender`, `subject`, `moneyCopper`, `read`, `daysLeft`, `expiresAt`, `items`), `totalOnServer` | ≤0.4.0 |
| `auctions` | auction house | `prices` (name → `id`, `minBuyout`, `minBid` per item, `quantity`, `listings`, `seenAt`, `seen` up to 5 points `{at, minBuyout}`, `source`; since 0.13.0 for gear also the item details: `type`, `subType`, `classID`, `subclassID`, `equipLoc`, `itemLevel`, `requiredLevel`, `quality`, `sellPrice`, `bindType`, `statTokens`, `dps`; since 0.19.2 a random suffix is its own entry under the name the auction window shows, with `suffix`, its own `statTokens`, the row's `tooltip` lines and, for weapons, `speed` and `damageMin`/`damageMax`, and the addon asks the client to load rows it has not loaded and reads them again; an item that never loaded has only `type`, `subType`, `classID`, `subclassID`, `equipLoc`; non-gear has no details), `owned` (own auctions: `id`, `name`, `count`, `bid`, `buyout`, `timeLeftSeconds`, `sold`), `ownedAt`, `lastSearchPartial` (`shown`, `total`), `lastVisitAt`. Prices merge between visits (up to 400 items). The addon neither searches nor lists anything | ≤0.4.0, details 0.13.0, suffix and tooltip 0.19.2 |
| `inspect` | Inspect window | map player name → `{name, class, classToken, race, level, guild, averageItemLevel, equipment, capturedAt}`; up to 30 players; only when the player clicked Inspect themselves | ≤0.4.0 |
| `buyback` | vendor (buyback list) | `npc`, `capturedAt`, `items` (`index`, `name`, `id`, `count`, `priceCopper` for the whole batch). An empty list with the vendor window open replaces the old record | 0.7.0 |
| `friends` | social window (`FRIENDLIST_UPDATE`) | `capturedAt`, `total`, `online`, `friends` (up to 100: `name`, `level`, `class`, `area`, `connected`, `notes`). Reads client memory, asks the server for nothing; does not remind about saving. Section absent = the window has not been read yet; `total = 0` = the list is empty | 0.7.0 |
| `guild` | guild window (`GUILD_ROSTER_UPDATE`, `PLAYER_GUILD_UPDATE`) | `capturedAt`, `name`, `rank`, `rankIndex` (the player's guild), `total`, `online`, `members` (up to 500: `name`, `rank`, `rankIndex`, `level`, `class`, `zone`, `online`), `onlineOnly` (`true` when the guild has more than 200 people and only those online were saved). List not loaded: the previous members are kept. `inGuild` (since 0.7.1): `true` when a guild is saved; a player without a guild has `{inGuild = false, capturedAt}` on every dump (only `IsInGuild()`, with no request for the list). Section absent = addon older than 0.7.1 or a guild whose window has not been opened yet. The addon does not call `GuildRoster` | 0.7.0 |
| `stable` | stable | `slots`, `pets` (`slot`, `name`, `level`, `family`, `loyalty`, `specialization`, `current`; since 0.7.1 `active = true` for pets from `C_StableInfo.GetActivePetList`, at the start of the list) | 0.6.0 |
| `questOffer` | quest offered by an NPC (`QUEST_DETAIL`) | `id`, `title`, `text`, `npc`, `rewards` (as in `quests[].rewards`); only the last window | 0.6.0 |
| `questTurnIn` | quest turn-in (`QUEST_COMPLETE`) | as above; only the last window | 0.6.0 |
| `questLookups` | the `/mcp quest <id or name>` command | list, newest first, up to 10: `query`, `at`, `quests` and `npcs` (entries as in `questDb`); a repeated query replaces its earlier answer; an answer with two empty lists = nothing found. Requires the quest database addon | 0.14.0 |
| `probe` | only after `/mcp probe` | `namespaces` (name → list of API functions), `globals`, `events` (which events the client accepted), `capturedAt`; diagnostics, the server does not write it to the history | ≤0.4.0 (`events` 0.5.0) |

## 6. Sections saved live

| Section | Contents | Since |
|---|---|---|
| `loot` | your own loot since the previous dump written to disk: `since` (Unix time of the start of the interval), `items` (`id`, `name`, `quality`, `count` = total items, `lastAt`; up to 100 distinct items). After login and `/reload` it starts from zero, so every dump in the history is a separate interval. Since 0.7.1 the section exists from login (with zeros) and has diagnostics `messagesSeen` (own loot messages), `messagesParsed` (recognized), `messagesSecret` (secrets that cannot be read) and `formats`. Since 0.12.0 `periods` (see below) carries the closed periods of earlier saves | 0.6.0 |
| `xpGains` | your own experience since the previous dump written to disk (like `loot`): `since`, `total`, `lastAt`, `kills`, `quests`, `other` (each `{count, xp}`). `kills` from `CHAT_MSG_COMBAT_XP_GAIN` messages with a creature name, `quests` from the `QUEST_TURNED_IN` event (xp > 0), `other` is XP without a name (e.g. exploration); quest XP is not counted twice. Since 0.7.1 the section exists from login (with zeros), has `messagesSeen`, `messagesParsed`, `messagesSecret` and `barXp` (growth of the XP bar from `PLAYER_XP_UPDATE`, also across a level-up; independent of chat). Since 0.9.0 messages are parsed with the client's own format strings (`COMBATLOG_XPGAIN_*`, `LOOT_ITEM_SELF*`), and the English patterns apply only when the client has none of them, so a localized client is no longer matched against English text. Since 0.12.0 `periods` carries the closed periods of earlier saves, like `loot` | 0.7.0 |
| `deaths` | `count` (number of deaths) and `recent` (up to 20: `at`, `zone`, `subZone`, `level`) from the `PLAYER_DEAD` event | 0.5.0 |
| `eventLog` | `entries`: timestamped events recorded while playing, oldest first, at most 300, entries older than 14 days dropped at login; carried over every save, so one save holds the whole session with real times. Every entry has `at` (Unix time) and `kind`: `level` (`level`, from `PLAYER_LEVEL_UP`), `money` (`copper` after the change, from `PLAYER_MONEY`; changes within 300 s of the last `money` entry overwrite its `copper` and keep its `at`), `questAccepted` (`questID`, `title` when the client knows it), `questTurnedIn` (`questID`, `xp`, `copper`), `zone` (`zone`, only when it differs from the previous `zone` entry), `equipment` (`slot`, `itemID`; absent when the slot is now empty, from `PLAYER_EQUIPMENT_CHANGED`) | 0.20.0 |
| `note` | the player's note from `/mcp note`: `text` (up to 500 characters) and `setAt`; absent when removed | 0.5.0 |

### Carried periods of `loot` and `xpGains` (0.12.0)

The game may save twice before the server reads the file (`/mcp reload`, then logout seconds later), and each save starts the
current period from zero. So the addon never assumes the server read a dump: when a login closes a period that has content,
the closed period is appended to the section's `periods` list and carried on in every following dump.

| Field | Meaning |
|---|---|
| `loot.periods`, `xpGains.periods` | list of closed periods, oldest first; absent when there is none. Each entry has the section's own fields as they were when the period closed (`since`, `items` with the 100-item cap, counters for `loot`; `since`, `total`, `kills`, `quests`, `other`, `barXp`, counters for `xpGains`) plus `until`: the `dumpedAt` of the save that closed the period (the next period's `since`) |

- Bounds: at most 20 periods (the newest) and none whose `until` is older than 7 days, checked at every login; a period
  without own loot (`loot`) or without XP from messages or the bar (`xpGains`) is not stored.
- The current-period fields (`since`, `items`, totals, counters) keep their meaning, so older servers ignore `periods`.
- A skipped logout dump of an unloaded client continues the current period and keeps `periods` unchanged.
- The server merges the periods that started at or after the older dump's `dumpedAt`, skips a carried period whose `since`
  equals a current period it already counted, and keeps one entry per `since` (the latest `until`); answers say `periodsMerged`.

## 7. Notes for the data reader

- Values that the client marks as "secrets" in combat go through `plain()`; if the client did not reveal them, the field is
  simply absent.
- Empty tables and lists are written the same way in Lua (`{}`), so the server treats them as an empty list.
- Time is always a Unix number (seconds), except `dumpedAtText`.
- Money is in copper (`*Copper`, `sellPrice`, `buyout`, `bid`).
- When a section fails, it is absent and an entry appears in `errors`; the server shows this in `wow_status` as `addonErrors`.
- A change of `schema` would mean backward incompatibility; for now version 2 applies.
