# wow-mcp

[![CI](https://github.com/Piekario/wow-mcp/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/Piekario/wow-mcp/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Piekario/wow-mcp?sort=semver)](https://github.com/Piekario/wow-mcp/releases/latest)
[![npm](https://img.shields.io/npm/v/@piekasz/wow-mcp?logo=npm)](https://www.npmjs.com/package/@piekasz/wow-mcp)
[![CurseForge downloads](https://img.shields.io/curseforge/dt/1824883?logo=curseforge&label=CurseForge)](https://www.curseforge.com/projects/1824883)
[![CurseForge version](https://img.shields.io/curseforge/v/1824883?logo=curseforge&label=addon)](https://www.curseforge.com/projects/1824883)
[![License: GPL-3.0-or-later](https://img.shields.io/badge/license-GPL--3.0--or--later-blue)](LICENSE)

[![Go](https://img.shields.io/github/go-mod/go-version/Piekario/wow-mcp?filename=server%2Fgo.mod&logo=go&logoColor=white)](server/go.mod)
[![Lua](https://img.shields.io/badge/addon-Lua-2C2D72?logo=lua&logoColor=white)](addon/WowMcp)
[![Node.js](https://img.shields.io/badge/npx-Node.js%2018%2B-339933?logo=nodedotjs&logoColor=white)](npm/wow-mcp)
[![MCP](https://img.shields.io/badge/protocol-MCP-6E56CF)](https://modelcontextprotocol.io)
[![Platforms](https://img.shields.io/badge/platforms-Windows%20%7C%20macOS%20%7C%20Linux-lightgrey)](#supported-game-versions)

An [MCP](https://modelcontextprotocol.io) server that lets an AI assistant in any MCP client read the state of your World of Warcraft
character: quests, bags, gear, talents, professions, reputation, auctions, progress over time, and more. It has two parts:

- **WowMcp** (addon, Lua): saves the character state to the game's SavedVariables file.
- **wow-mcp** (MCP server, Go): reads that file and exposes it as 52 `wow_*` tools, 14 prompts and read-only resources.

> **License:** [GPL-3.0-or-later](LICENSE) with an additional term that the author attribution must be kept in every copy and
> modified version (see [NOTICE](NOTICE)). Original author: Piekasz (GitHub: Piekario).

## Read-only, and Blizzard's Terms of Service

Everything is read-only. The addon uses the official API and clicks nothing in the game; it never automates gameplay.
The server opens one text file from the `WTF` directory (`WowMcp.lua` in SavedVariables) and does not touch the game
process, its memory, network traffic or pixels. The server writes only in its own directory (never in the game directory):
the dump history, an optional log and the goals file, which is changed by a single tool, `wow_goal_set`.
What the addon and the server do, and what they will never do, is described in [docs/TOS_COMPLIANCE.md](docs/TOS_COMPLIANCE.md).
That document is a description by the project author, not an approval from Blizzard and not legal advice.

## Supported game versions

- **World of Warcraft: Forever** (client 1.60.1, Interface 16001): the only client this project has been verified on. Addon 0.5.0 with
  server 0.6.0 and addon 0.7.0 with server 0.9.0 ran in a real client. Later versions (see [CHANGELOG.md](CHANGELOG.md)) are
  tested on API mocks that match a real `/mcp probe` result and have not yet been run in the game.
- **Other clients** (retail, Classic Era, Wrath, Cataclysm Classic, ...): **untested.** The addon has compatibility paths for both
  the modern (`C_*` namespaces) and the classic function sets, but they are exercised only against mocks in `addon/tests`. Expect that
  some section may fail on another client; a failing section is reported (`addonErrors`, `/mcp status`) and does not break the rest.
  The `## Interface:` number in `WowMcp.toc` is for Forever; on another client tick "Load out of date AddOns" or edit that number.
- **Localized (non-English) clients** are supported on a best-effort basis: the addon and server are being made to use locale-independent
  identifiers (addon 0.9.0 / server 0.11.0), but this too is tested only on mocks, not in real localized clients.
  Item and ability names in responses are whatever your client returns.

## Quick start (about 60 seconds)

### With npx (Windows, macOS, Linux; needs Node.js 18+)

```sh
npx -y @piekasz/wow-mcp install-addon      # copies the WowMcp addon into the game (finds the game; or pass --wow-dir / --client)
```

Restart the game completely, type `/mcp reload` once, then add the server to your MCP client:

```json
{
  "mcpServers": {
    "wow": {
      "command": "npx",
      "args": ["-y", "@piekasz/wow-mcp"],
      "env": { "WOW_DIR": "C:\\Program Files (x86)\\World of Warcraft" }
    }
  }
}
```

npm installs only the prebuilt server for your platform, so you need neither Go nor a manual download. With npx the dump history
lives in your user data directory (`%LOCALAPPDATA%\wow-mcp`, `~/Library/Application Support/wow-mcp` or `~/.local/share/wow-mcp`).

### From a release archive

Download the archive for your platform from the releases page (`wow-mcp-v<server>-addon-<addon>-<os>-<arch>`, with `SHA256SUMS`),
or build it yourself (see [Building from source](#building-from-source)). Every archive holds the server binary, the `WowMcp`
addon, the installer, this README, the changelog and `docs/`. The installers copy the addon into
`<game>/<client>/Interface/AddOns/WowMcp` (the client folder is `_forever_` by default), keep a backup of the previous copy,
install the server, run `--check` and print the MCP configuration. They write nothing else in the game directory (never `WTF`).

### Windows

1. Unzip `wow-mcp-...-windows-amd64.zip` (or `arm64`) anywhere, e.g. `C:\Tools\wow-mcp`.
2. In PowerShell inside that folder (use `-WhatIf` first to see the plan):

   ```powershell
   powershell -ExecutionPolicy Bypass -File .\install.ps1 -WowDir "C:\Program Files (x86)\World of Warcraft"
   ```

   `-WowDir` is the folder that contains `_forever_`. Without it the script uses the `WOW_DIR` variable, then a built-in default
   that is probably not your path. The server ends up in `bin\wow-mcp.exe` next to the script.
3. Add the server to your MCP client (see [Client configuration](#client-configuration)) using that `bin\wow-mcp.exe` path.

### macOS

```sh
tar xzf wow-mcp-v*-darwin-arm64.tar.gz      # or darwin-amd64 on an Intel Mac
cd wow-mcp-v*-darwin-arm64
./install.sh --dry-run                      # shows the plan, changes nothing
./install.sh                                # default game directory: /Applications/World of Warcraft
```

The binary is not signed or notarized. If Gatekeeper blocks it, remove the quarantine flag once:
`xattr -d com.apple.quarantine ~/.local/share/wow-mcp/wow-mcp`.

### Linux (Wine, Lutris, Bottles, Steam Proton)

```sh
tar xzf wow-mcp-v*-linux-amd64.tar.gz       # or linux-arm64
cd wow-mcp-v*-linux-amd64
./install.sh --dry-run
./install.sh --wow-dir "$HOME/Games/battlenet/drive_c/Program Files (x86)/World of Warcraft"
```

`install.sh` looks in the usual prefixes (`~/.wine`, `~/Games/*`, Bottles, Lutris, Steam Proton `compatdata`) and uses the game directory it
finds when there is exactly one; if it finds none or several, pass `--wow-dir`. The server itself is a native Linux binary that only reads a
file, so it does not run under Wine: it needs the path of the game directory as seen by Linux (see [Where the server looks](#where-the-server-looks-for-the-dump)).

### `install.sh` options

| Option | What it does |
|---|---|
| `--dry-run` | print the plan, change nothing |
| `--wow-dir DIR` | game directory (default `$WOW_DIR`, then auto-detection) |
| `--client NAME` | client folder inside the game directory (default `_forever_`) |
| `--install-dir DIR` | where the server, its history and backups go (default `~/.local/share/wow-mcp`, or `$WOW_MCP_HOME`) |
| `--binary FILE` / `--build` | use another binary / build from `server/` (source checkout, needs Go) |
| `--keep N` | addon backups and old binaries to keep (default 2) |
| `--skip-check` | do not run `wow-mcp --check` at the end |
| `--rollback` | restore the previous addon backup and the previous server binary |

The addon consists of dozens of files listed in `WowMcp.toc`: copy it with the installer, not by hand, so that obsolete files are
removed. After an install that adds or removes addon files, close the game fully and start it again (`/reload` does not re-read the `.toc`).

### First dump

Start the game, tick WowMcp on the character selection screen and type `/mcp reload`: the addon dumps the character state and
reloads the interface, and the game writes `<game>/_forever_/WTF/Account/<ACCOUNT>/SavedVariables/WowMcp.lua`.
Then `wow-mcp --check --wow-dir "<game directory>"` prints the file path, the character name and the dump time.
A checklist for the first run: [docs/FIRST_RUN.md](docs/FIRST_RUN.md).

### Where the server looks for the dump

The server needs the **game directory**, the folder that contains the client folder (`_forever_`). Pass it with `--wow-dir` or the `WOW_DIR`
environment variable. It searches `<dir>/WTF/Account/*/SavedVariables/WowMcp.lua` and `<dir>/_forever_/WTF/Account/*/SavedVariables/WowMcp.lua`
(so the directory itself or its `_forever_` subfolder both work) and takes the most recently modified file when there are several accounts.
Other client folders (`_retail_`, `_classic_`, ...) are not searched automatically: for those pass the file itself with `--file`.

| OS | Typical `--wow-dir` |
|---|---|
| Windows | `C:\Program Files (x86)\World of Warcraft` (or wherever the Battle.net launcher installed it) |
| macOS | `/Applications/World of Warcraft` |
| Linux, Wine / Lutris | `~/Games/battlenet/drive_c/Program Files (x86)/World of Warcraft` or `~/.wine/drive_c/Program Files (x86)/World of Warcraft` |
| Linux, Steam Proton | `~/.steam/steam/steamapps/compatdata/<appid>/pfx/drive_c/Program Files (x86)/World of Warcraft` |

If your layout is unusual, `--file "<path>/WowMcp.lua"` skips the search entirely. The server never writes inside the game directory:
its history, log and `goals.json` go next to the binary (or to the paths given by `--history-dir`, `--log`, `--goals-file`).

## Client configuration

wow-mcp is a standard MCP server over stdio, so it works with any MCP client. Most clients take a JSON entry under `mcpServers`
(the file and menu differ per client; see its documentation). Use the absolute path of the installed binary (the installer prints a
ready snippet), or `npx` as in the [quick start](#with-npx-windows-macos-linux-needs-nodejs-18):

```json
{
  "mcpServers": {
    "wow": {
      "command": "/path/to/wow-mcp",
      "args": ["--wow-dir", "/path/to/World of Warcraft"]
    }
  }
}
```

On Windows use escaped backslashes: `"command": "C:\\Tools\\wow-mcp\\bin\\wow-mcp.exe"`, `"args": ["--wow-dir", "C:\\Program Files (x86)\\World of Warcraft"]`.
Instead of the arguments you can set the `WOW_DIR` environment variable. Restart the client completely after changing its configuration.
Logs go to stderr (and to `--log` if given), never to stdout.

## Usage

| In-game command | What it does |
|---|---|
| `/mcp reload` | dumps the state and immediately writes it to disk (`ReloadUI()`) |
| `/mcp` | a dump only; it reaches the disk on the next `/reload` or logout |
| `/mcp status` | addon version, time of this character's last dump, how long it took to collect, disabled sections and errors |
| `/mcp panel` | shows or hides the WowMcp panel |
| `/mcp played on` / `off` | whether to ask the game for play time (`/played`) after login, from which the XP rate is computed |
| `/mcp note <text>` | a note for the assistant in the dump, e.g. what you plan to do today; `/mcp note` alone removes it |
| `/mcp off <section>` / `/mcp on <section>` | stop collecting a section (it disappears from the dump) or collect it again; `/mcp off` alone shows the disabled ones and all names |
| `/mcp minimap on` / `off` | the minimap button (on by default): a left click shows or hides the panel, saves nothing; it can be dragged |
| `/mcp options` | opens the WowMcp page in the game's options (Interface / AddOns) with the same switches as these commands: sections, minimap button, compact panel, AH tooltip, played time; changes settings only |
| `/mcp compact on` / `off` | the panel in a single line (dump age and the Save button); off by default |
| `/mcp ahtip on` / `off` | whether to show an "AH: price (age)" line in the item tooltip with the last saved auction house price (on by default; text in the tooltip only) |
| `/mcp probe` | diagnostics: a list of the client's API functions and the events the client accepted; also does `/mcp reload` |
| `/mcp help` | list of commands |

Alias: `/wmcp`. Deaths (the counter and recent places) are recorded by the addon on its own.

State is also saved on every `/reload` and logout. The addon captures a trainer's offer at the moment the trainer window is opened (it remembers up to the 10 most recent trainers and vendors), so the order is: open the trainer, type `/mcp reload`, ask your assistant. The professions window (recipes) and the mailbox work the same way.

### In-game panel

After login a small WowMcp panel appears in the upper right corner. It shows how many minutes ago the last dump was (green up to an hour, yellow up to two, then red), the number of free bag slots and section errors. Hovering over the panel shows the section errors. The **Save** button does exactly the same as `/mcp reload`; when a window (trainer, auctions and others) has produced new data, the button is highlighted. Next to the panel there is a minimap button (a left click shows or hides the panel, it never saves; `/mcp minimap off` hides it), and `/mcp compact on` collapses the panel to a single line. The panel can be moved with the mouse (the position is remembered) and closed with the X; `/mcp panel` brings it back.

The panel works only on click: the highlight is a reminder, and the addon never saves or reloads anything on its own.

### Play time

After login (and after every `/reload`) the addon asks the game for play time, like the `/played` command. That is why a "Total time played" line appears in chat. This way the XP and gold rate is computed from the time actually spent in the game, and offline breaks do not lower it. If that line bothers you, type `/mcp played off`; the rate will then be computed from the time between dumps.

### Tools

| Tool | What it returns |
|---|---|
| `wow_status` | server and addon versions, file path, list of characters, data age, the player's note, disabled sections and `alerts` (expiring mail and auctions, damaged gear, full bags, slow dump, addon errors) |
| `wow_doctor` | one diagnostic report: ordered checks (dump file, account, age, addon version, section errors, windows never opened, client language, history) with a fix for each and the next step |
| `wow_character` | level, class, gold, XP, zone and coordinates, `resting`, Hearthstone location (`bindLocation`), professions, talents, note, deaths and PvP (`pvp`: honor, kills, rank) |
| `wow_quests` | the quest log with objective progress |
| `wow_bags` | bags, free slots, items |
| `wow_equipment` | equipped gear with durability and the game's equipment sets (`equipmentSets`, with `isEquipped`) |
| `wow_spells` | known abilities and ranks |
| `wow_pet` | pet: level, family, happiness, loyalty, training points, action bar, abilities; `rankReview` (missing ranks from Beast Training and trainer offers), `trainingPlan` (what fits in the free points) and `stable` (stabled pets versus the active one) |
| `wow_trainer` | the offer of the last trainer (or a remembered one chosen by `npc`; list in `known`) with prices and a `canAfford` field |
| `wow_reputation` | reputation with progress in percent and the amount missing to the next level; filters `query`, `standing` |
| `wow_bank` | the bank from the last visit with its vendor value; filters `query`, `type`, `minQuality`, `sellable` |
| `wow_vendor` | the offer of the last vendor (or a remembered one chosen by `npc`; list in `known`) with `canAfford`, `levelOk` and a comparison with equipped gear (`vsEquipped`), plus the `buyback` list (assumption: the client loses it after logout); filters `query`, `type`, `minQuality`, `affordable`, `usable`, `equippable` |
| `wow_compare_item` | compares an item from bags, bank or a vendor with the equipped one: differences in stats, DPS, armor and item level; `weights` (or `weights.json`) adds `weightedDelta` |
| `wow_changes` | what changed since the previous dump: gold, XP, items, gear, quests, reputation, new abilities, your own loot (`loot`) and XP by source (`xpSources`: kills, quests, other); `steps` or `sinceHours` reaches further back |
| `wow_quest_plan` | quests grouped by zone with the in-game difficulty color (red, orange, yellow, green, gray), objective progress and warnings: too hard, gray, failed, group, full log |
| `wow_progress` | rates from the dump history: XP and gold per hour in the current session and over the whole history, quests turned in, level-ups, how much XP to the next level and how long that takes; `xpSources` splits XP into kills, quests and other |
| `wow_account` | all characters on the account: level, class, gold (and total), XP, rested, professions, free slots, play time, mail, ready cooldowns |
| `wow_find_item` | looks for an item across all characters: bags, gear, bank, mail |
| `wow_cooldowns` | long cooldowns (Hearthstone, abilities, professions) with time remaining; `all` shows all characters |
| `wow_recipes` | recipes from the professions window (including Beast Training): color, how many can be made now, materials in bags and bank, what is missing; filters `profession`, `query`, `skillUp`, `craftable` |
| `wow_mail` | mail from the last mailbox opening: gold, COD, attachments, what is about to expire |
| `wow_legacy` | Forever Legacy: unspent and spent Legacy Points, the three perk trees with descriptions, what can be learned now and the open Legacy challenges |
| `wow_talents` | talent layout (e.g. 11/0/0), free points, whole trees with descriptions from the game and a list of talents you can spend a point on now |
| `wow_auctions` | auction house prices saved during your searches, a trend from recent visits, your auctions, an assessment of what from your bags and bank is worth listing instead of selling, and for gear the comparison with what you wear (`vsEquipped`, `canAfford`, `levelOk`; filters `type`, `equippable`, `affordable`, `usable`) |
| `wow_group` | party or raid composition: classes, roles, levels, who is offline or far away; for players inspected with the Inspect window, average item level and full gear (`player`) |
| `wow_dungeons` | dungeons that fit your level, those a few levels ahead, quests from your log for each dungeon and instance lockouts |
| `wow_export` | a character summary to paste into Discord (Markdown), a forum (BBCode) or as plain text; `short`, `gold`, `english` |
| `wow_search` | text search across the whole dump of one character (quests and their rewards, quest windows at NPCs, items, abilities, trainers and vendors, stable, loot, recipes, mail, auctions, macros, flights); hits grouped by section |
| `wow_rested` | rested XP: how much there is, how many levels that is, whether it covers the rest of the level and when it will be full after logging out in an inn (Classic rules, approximate) |
| `wow_gold_history` | gold over time from the dump history: day by day, the three largest income and expense items; `days` |
| `wow_leftovers` | quest items in bags and bank whose quest is no longer in the log; with a confidence rating, removes nothing |
| `wow_ammo` | ammunition: stock, consumption per hour from the history, how many hours it lasts and how much to buy (`hours`) |
| `wow_reagents` | stocks of ammo, food, bandages, potions and reagents, consumption per hour from the history and how long they last (`lowStock`) |
| `wow_journal` | a session journal from the history (levels, XP and its sources, gold, quests, notable items, loot, deaths, zones) and records; `sessions`, `markdown=true` returns ready text in the `text` field (the server itself does not write `.md` files) |
| `wow_bag_plan` | what to do with full bags: keep, bank, sell, auction, with the reason and totals; destroys nothing |
| `wow_quest_rewards` | an evaluation of choice rewards for quests against your gear (a quest being turned in or offered by an NPC at the top, then the log): slot, differences, score, vendor value, `best` with a justification; `quest` (name or ID), `weights`. Repeated reward lists across several quests get `suspect` and no recommendation |
| `wow_shopping_list` | what to buy before leaving town: trainer (from all remembered ones), repair, ammo, pet food, running-low materials, who sells it and for how much, a budget in priority order; `returnToTown` says when to return to town; `hours` |
| `wow_goals` | the player's goals from `goals.json`: current state, how much is missing, percent and an ETA from the `wow_progress` rate |
| `wow_goal_set` | adds or removes a goal (`level`, `gold`, `reputation`, `profession`, `spell`, `item`); **the only tool that writes**, and only to the server's goals file |
| `wow_timeline` | a character timeline from the dump history: level-ups, new abilities, gear changes, quests turned in, deaths, profession thresholds; `kind`, `days`, `limit`. An event has the time of the first dump in which it is visible |
| `wow_buffs` | active buffs and debuffs and missing standing class buffs (e.g. Aspect, Arcane Intellect); time remaining from `expiresAt` (addon 0.7.0 records `expiresAt` and `duration`) |
| `wow_flights` | known flight points matched against the zones where you have quests (zone from the flight point's `zone` field from addon 0.7.0, and for older dumps guessed from the name) |
| `wow_session_rating` | the rate of the last session (XP, gold balance, quests per hour) against the median of earlier sessions |
| `wow_compare_characters` | two characters of the account side by side: level, gold, play time, professions, ilvl and gear slot by slot (`a`, `b`) |
| `wow_alt_mail` | what to mail between characters: materials from bags and bank that are ingredients of the other character's recipes (`from`, `to`, `query`); a lead based only on the kind of material is flagged as weak |
| `wow_account_crafting` | what can be crafted from the whole account's materials, who holds what and where to send it from (`character`, `profession`, `query`, `craftable`, `limit`) |
| `wow_make_or_buy` | craft or buy at the auction house: ingredient cost versus the price of the finished item; a missing price is `unknown`, never zero (`profession`, `query`, `sort`, `limit`) |
| `wow_quest_route` | a suggested order of zones with quests (current zone, then by the number of quests ready to turn in and of all quests) with known flight points; this is not the shortest route, the server does not know the world map |
| `wow_quest_lookup` | where a quest is picked up, done and turned in: NPC or object, zone, map coordinates, objective targets, next quest of the chain and a one-line `nextStep`; the data is read in game from the quest database the player has installed (QuestieDB, what Questie shows), never from memory or the quest text (`quest`: ID or a fragment of a quest or NPC name; `/mcp quest <name or ID>` in game adds any other quest) |
| `wow_guide` | the next steps of the player's Guidelime leveling guide (for example TUGs) in guide order, as readable text with quest names, states and places |
| `wow_social` | friends and guild from the most recently opened windows: who is online, level, class, zone, rank, counts and the age of the record; `includeOffline`. The addon does not ask the game server for these lists on its own, so you must open them in the game (PvP is in `wow_character`) |
| `wow_section` | any dump section by name, without filters: `actionBars`, `macros`, `completedQuests`, `flightPaths`, `stats`, `auras`, `currencies`, `probe`, `loot`, `xpGains`, `buyback`, `friends`, `guild`, `vendors`, `trainers`, `stable`, `questOffer`, `questTurnIn` and the rest |

`wow_character` additionally shows currencies and tokens and play time, `wow_bags` the value of junk (Poor quality) and ammo, and `wow_equipment` a durability summary with the repair cost at the last vendor that repairs. `wow_status` hints when the dump comes from an older addon version. Example questions for every tool: [docs/HOW_TO_ASK.md](docs/HOW_TO_ASK.md).

Every tool accepts an optional `character` parameter. Without it, the character from the last dump is used.

### Ready-made prompts

Every prompt takes an optional `character`. In your MCP client they are in the `wow` server's prompt list (the "+" button or the attachments menu). Each one collects the data it needs from the dump by itself:

| Prompt | What it does |
|---|---|
| `session_plan` | a plan for the session: quests by zone, what to turn in, trainer, bag space, changes since the previous dump, when to return to town (`returnToTown`) |
| `trainer_shopping` | what to buy from the trainer with your current gold and what to put off |
| `what_to_sell` | what from bags and bank to sell and what to keep for quests and professions |
| `gear_upgrades` | which items from bags, bank and a vendor are an upgrade |
| `session_summary` | what the last session gave: XP, gold, rate, quests turned in, how much to the next level |
| `account_overview` | all characters: gold, ready profession cooldowns, expiring mail, who is worth logging in with now |
| `profession_progress` | how to level a profession from what you already have in bags and bank |
| `talent_planning` | where to spend the next points and what build to aim for, based on the tree from the game |
| `what_to_auction` | what to list at the auction house, at what price, and what to hand to the vendor |
| `dungeon_finder` | which dungeons now, which quests to pick up and how to prepare |
| `before_logout` | what to do before leaving: an inn for rested XP, mail, auctions, repair, bag space, warnings from `wow_status` |
| `first_run` | an introduction after installation or for a new character: what the assistant sees, which windows to open, what to ask about |
| `week_in_game` | a summary of the last 7 days from the history: levels, gold, play time, the best session and events |
| `pet_care` | the state of a Hunter's pet: happiness, loyalty, training points, diet versus food in bags (by item subtype), missing ranks and a training points plan, comparison with the stable |

`session_plan` now uses the quest difficulty rating, the XP rate, cooldowns, gear durability and mail.

### MCP resources

Besides tools, the server exposes the dump as read-only resources: `wow://index` (a list of characters and the addresses of their sections,
with the most recently dumped character marked `last`) and the template `wow://<character>/<section>` (JSON of the same section that
`wow_section` returns). Resources can be attached in a conversation without calling a tool; the list is registered once at server start.

### Talents

The addon saves whole talent trees, including rank 0 talents, with descriptions from the game. The trees in Forever differ from Classic (they contain new talents), so the server has no built-in "popular builds" and does not advise from Classic memory. Advice is based on what the game recorded.

### Auction house

The addon neither searches nor lists anything at the auction house. It saves only what you search for yourself and see in the results window: the lowest buyout and bid price per item, the number of items and auctions. Prices accumulate between visits, so comparisons grow over time. Opening the tab with your auctions saves those too. The order: search at the auction house for what you want to sell, type `/mcp reload`, ask your assistant (or use the `what_to_auction` prompt). In the classic window the current results page is saved, so it is worth sorting by price.

### Group, raid and Inspect

The party composition is saved on every dump. The gear of other players is saved when you click Inspect on them yourself (the addon sends no queries about other players). The 30 most recently inspected are remembered.

### Dungeons

`wow_dungeons` takes level ranges from the in-game group finder if the client provides them. Otherwise it uses the dungeon table from Classic, whose ranges are approximate; the response then says `source: classic`. Quests for a dungeon are those that sit under the dungeon's header in the log or mention it in the title or objectives.

### Multiple characters

SavedVariables are shared across the whole account, so the file contains a dump of every character that did `/mcp reload` or logged out with the addon enabled. It is enough to log in once with each alt. `wow_account`, `wow_find_item` and `wow_cooldowns` with `all: true` then show all characters at once, and every tool accepts `character` to ask about a specific one.

### Progress statistics

`wow_progress` computes rates from the dump history. A session is a series of dumps without a longer break: 30 minutes offline by play time or 120 minutes between dumps when there is no play time (the `breakMinutes` parameter changes this). The more often you do `/mcp reload` (or click Save), the more accurate the numbers.

### Dump history

The server remembers every dump it reads in the `history` directory next to the server binary (with `install.sh`: `~/.local/share/wow-mcp/history`; on Windows: `bin\history`) (one JSON file per dump, by default the 50 most recent per character; dumps older than 7 days are gzipped as `.json.gz`, and one character's directory has a 200 MB limit, see `--history-max-mb` and `--history-compress-days`). `wow_changes` computes differences from this history. A dump enters the history when the server reads it, that is, on the first question to the assistant after `/mcp reload`. Two `/mcp reload` with no question in between therefore give a single entry.

### Stale data

When a dump is more than 2 hours old, every tool starts its response with a `WARNING:` line giving the data age and asking for `/mcp reload`. Sections from windows (`trainer`, `vendor`, `bank`, `flightPaths`) have separate `capturedAt` and `capturedAgeMinutes` fields, because they may come from an old visit.

### Goals and stat weights

`wow_goal_set` saves goals in the `goals.json` file (by default next to the history directory, or the one given by `--goals-file`). This is the server's only write besides the history and the log; the server never writes in the game directory. Gold in the file is copper. Example:

```json
{"version":1,"goals":[
  {"id":"g1","type":"level","character":"Thrall","target":25},
  {"id":"g2","type":"reputation","character":"Thrall","faction":"Darnassus","standing":"Revered"},
  {"id":"g3","type":"item","character":"Thrall","item":"Linen Cloth","target":40,"note":"for bandages"}
]}
```

The optional `weights.json` file (next to the goals file) sets stat weights for `wow_compare_item`, `wow_vendor` and `wow_quest_rewards`; without it (and without the `weights` argument) the server gives only the stat differences, with no point score; the source of the weights is in the response (`scoring`). Weights from the `weights` argument take precedence, then the character's entry, then the class's:

```json
{"characters":{"Thrall":{"agility":1,"attackPower":0.5}},"classes":{"HUNTER":{"agility":1}}}
```

### Server flags

| Flag | Default | What it does |
|---|---|---|
| `--wow-dir` | the `WOW_DIR` variable | game directory |
| `--file` | | full path to `WowMcp.lua` instead of searching |
| `--history-dir` | `history` next to the exe | history directory; `--history-dir ""` turns the history off; a path in the game directory is rejected |
| `--history-keep` | 50 | how many dumps to keep per character (`wow_progress` is computed from them) |
| `--history-max-mb` | 200 | size limit of one character's history in MB; the oldest dumps disappear first, the newest stays; 0 = no limit |
| `--history-compress-days` | 7 | dumps older than this many days are gzipped (reading works as before); a negative value turns compression off |
| `--stale-minutes` | 120 | after how many minutes to warn about stale data; 0 turns it off |
| `--log` | | a file to which the server appends errors and startup information; a relative path is counted from the exe directory, a path in the game directory is rejected |
| `--goals-file` | `goals.json` next to the history | goals file (`wow_goals`, `wow_goal_set`); a relative path is counted from the exe directory, a path in the game directory is rejected |
| `--max-response-kb` | 48 | answers larger than this become a shortened envelope with a hint how to narrow the request; 0 = no cap |
| `--account` (`WOW_ACCOUNT`) | newest dump | pins the account folder under `WTF/Account` when there are several |
| `--client` (`WOW_CLIENT`) | newest dump | pins the client folder (`_forever_`, `_classic_`, ...) inside the game directory |
| `--check` | | checks the dump file and exits |
| `--version` | | prints the version and exits |

## When something does not work

- **Start with `wow_doctor`.** Ask the assistant to run it: it checks the dump file, versions, data age, section errors and windows never opened, and says what to do next.
- **The addon is marked "out of date".** Type `/run print(select(4, GetBuildInfo()))` in the game and put the printed number in the `## Interface:` line in `WowMcp.toc`. The installers warn about this in the Interface check step.
- **`/mcp status` shows section errors.** The addon was tested outside the game on API mocks, so in a real client some function may be named differently. An error in one section does not break the others, and its text is also in the tool responses in the `snapshot.addonErrors` field and in `alerts` in `wow_status`. Paste it into an issue; the fix is usually one line. `/mcp probe` helps.
- **The dump is slow to collect.** `/mcp status` shows the slowest sections; turn off unneeded ones with `/mcp off <section>`.
- **The server does not find the file.** Run `wow-mcp --check --wow-dir "..."` (`wow-mcp.exe` on Windows). The message says what is missing. You can also point to the file directly with the `--file` flag. When the game happens to be writing the file, the server retries the read a few times on its own.
- **Your MCP client does not see the tools after editing the configuration.** The app reads the configuration only at startup, so a full restart is needed.
- **Rolling back to the previous version:** `install.sh --rollback` (macOS, Linux) or `install.ps1 -Rollback` (Windows), then restart the MCP client and `/reload` in the game.

## Building from source

Requires Go 1.25 or newer (the official MCP SDK requires it). For the addon tests: Python 3 with `lupa` (`pip install lupa`) and `luacheck`.
The helper scripts are POSIX shell; on Windows run them from Git Bash or WSL, or use the plain `go` commands below.

```sh
make test      # go vet, go test, addon harness in every mode vs the committed fixtures, luacheck
make build     # the server for this platform -> bin/wow-mcp (or bin/wow-mcp.exe)
make dist      # cross-compile 6 platforms and write archives + SHA256SUMS into dist/ (ignored by git)
```

`make dist` reads the versions from the code (`serverVersion` in `server/main.go`, `## Version:` in the `.toc`), so archive names never need editing.
Pushing a `v*` tag runs the same steps in `.github/workflows/release.yml` once the repository is on GitHub. Without make:

```sh
cd server
go vet ./...
go test ./...                                                         # including the golden response tests
go test -run Golden -update .                                         # refresh the golden files, then review the diff of testdata/golden
go test -run=^$ -fuzz=FuzzParse -fuzztime=75s ./internal/savedvars    # parser fuzzing
go build -trimpath -ldflags "-s -w" -o ../bin/wow-mcp .
```

From a source checkout `./install.sh --build` builds the server and installs it together with the addon.

The out-of-game addon test runs the addon code in Lua 5.1 with API mocks and writes the result in the SavedVariables format, which then
serves as the server's test data:

```sh
python3 addon/tests/run_addon.py modern  server/testdata/WowMcp.lua
python3 addon/tests/run_addon.py legacy  server/testdata/WowMcp_legacy.lua
python3 addon/tests/run_addon.py forever server/testdata/WowMcp_forever.lua
```

## Community and contributing

- **Questions and ideas:** [Discussions](https://github.com/Piekario/wow-mcp/discussions). Where to get help: [SUPPORT.md](SUPPORT.md).
- **Bugs and feature requests:** [Issues](https://github.com/Piekario/wow-mcp/issues/new/choose) (please include `wow_doctor` output and `/mcp status`).
- **Contributing code, docs or tests:** see [CONTRIBUTING.md](CONTRIBUTING.md). Fork, branch, `make test`, pull request; CI must pass on Linux,
  macOS and Windows. Reports from game clients other than WoW: Forever (`/mcp probe` output) are especially welcome.
- **Security problems:** report privately, see [SECURITY.md](SECURITY.md).
- Everyone taking part follows the [Code of Conduct](CODE_OF_CONDUCT.md).

### Structure

```
addon/WowMcp/    addon: Core, Compat, Items, Store, Events, Timing, Snapshot (section registry),
                       Sections/* (one dump section per file), Panel, MinimapButton, Commands; order in .toc = order of sections
addon/tests/           addon test on API mocks
server/main.go         flags, --check, --version, log; composition root
server/internal/
  savedvars/           Lua literal parser (SavedVariables format), with a fuzz test
  snapshot/            dump, account character database, safe reads, formatting, Freshness (clock and staleness threshold)
  storage/             finding and reading the file, dump history, the server's own files, game directory guard
  game/<context>/      domain logic: addon, alerts, ammo, auctions, bagplan, character, consumption, cooldowns, dungeons,
                       export, goals, group, items, journal, leftovers, loot, mail, pet, professions, progress,
                       questrewards, quests, reagents, reputation, rested, search, shopping, talents, trainer
                       (since 0.8.0 also accountcraft, altmail, buffs, compare, flights, makeorbuy, sessionrating, timeline;
                       since 0.9.0 questroute, social, xpgains)
  mcpserver/           MCP adapter: tool registry (tools.go) and prompt registry (prompts.go)
  fixtures/            test data, for tests only
server/golden_test.go  golden response tests; golden files in server/testdata/golden/
install.ps1, install.sh   deployment and rollback (Windows; macOS and Linux)
Makefile, scripts/     test, build and release archive tooling
docs/                  first-run checklist, how to ask your assistant, ToS compliance, dump format
bin/                   built or installed server binaries (outside git)
```

The `game/*` packages import only `snapshot` and other `game/*`: no MCP SDK, no file system. A new tool is a function
in `game/<context>`, an entry in `mcpserver/tools.go` and cases in `golden_test.go` (the test fails when a tool, prompt or argument
has no case). A new addon section is a file in `Sections/`, a registry call and a line in `.toc`.

## Documentation

- [CHANGELOG.md](CHANGELOG.md): what changed in each version
- [ROADMAP.md](ROADMAP.md): plan and rejected ideas
- [docs/FIRST_RUN.md](docs/FIRST_RUN.md): checklist after deployment
- [docs/HOW_TO_ASK.md](docs/HOW_TO_ASK.md): example questions for your assistant
- [docs/TOS_COMPLIANCE.md](docs/TOS_COMPLIANCE.md): what the addon and server do, and what they will never do
- [docs/DUMP_FORMAT.md](docs/DUMP_FORMAT.md): the sections and fields of the dump
- [docs/PUBLISHING.md](docs/PUBLISHING.md): what the owner has to do to publish the project
- [TASKS.md](TASKS.md): the roadmap as a checklist

## Principles the project follows

1. The addon only reads state through the official API.
2. No actions in the game without a player click.
3. The server reads only the file written by the game; it writes only the history, log and goals file (through `wow_goal_set`) in its own directory, never in the game directory.
4. Zero contact with the game process, its memory and network traffic.

## What the addon dumps (v0.10.0)

On every `/mcp reload`: character, statistics, buffs, quest log, IDs of completed quests, bags and gear
(with stats and the full item description, and ammo with the item count), abilities, talents, professions
and weapon skills, pet (action bar and learned abilities), `equipmentSets` (equipment sets), `pvp` (honor and kills), aura duration (`expiresAt`, `duration`), action bars, reputation, macros,
`currencies` (currencies and tokens), `cooldowns` (cooldowns longer than 90 s, e.g. Hearthstone), `played` (play time),
`group` (party or raid composition) and `dungeons` (dungeons from the group finder and instance lockouts). Talents have
whole trees with descriptions. The character also has zone, subzone, coordinates, `resting` and `bindLocation`. The dump carries `timings`
(collection time) and `disabledSections`.

After opening a window in the game (and then `/mcp reload`): `trainer` and `vendor` (the last offer, and in `trainers` and `vendors` up to the 10 most recent NPCs; the vendor with repair cost),
`bank` (bank), `flightPaths` (flight map), `recipes` (professions window; each profession separately, including Beast Training
and Enchanting), `mail` (mailbox), `auctions` (prices from auction house search results and your auctions),
`stable` (stabled pets), `buyback` (the vendor's buyback list), `friends` (friends) and `guild` (guild; both from client memory after opening the window, the addon asks the game server for nothing), `questOffer` and `questTurnIn` (the quest window at an NPC: offer and turn-in, with choice rewards) and `inspect` (gear of players inspected with the Inspect window). These sections hold the state from the last visit. Saved live
are `deaths` (deaths), `loot` (your own loot since the last save), `xpGains` (your own XP since the last save: kills, quests, other) and `note` (the note from `/mcp note`). The item tooltip shows an "AH" line with the last saved price (you can turn it off: `/mcp ahtip off`). Quest rewards have the quest ID, and talents the full grid with descriptions. Recipes under
collapsed headers in the professions window do not reach the dump.
