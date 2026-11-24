# First run in a real client

A checklist to follow after deploying addon 0.8.0 and server 0.10.0. Goal: verify what the Forever client really returns. On 2026-11-15
in client 1.60.1 this checklist was completed by versions 0.5.0 / 0.6.0 (dump 18 ms) and 0.7.0 / 0.9.0 (zero section errors, dump 29 ms).
The 0.7.1 / 0.9.1 fixes were tested on those dumps and on mocks matching `/mcp probe`; item 3c checks them.

## 1. Installation

Unpack the release archive for your platform (or use a source checkout) and run the installer from that folder. Add `-WhatIf` (Windows)
or `--dry-run` (macOS, Linux) first to see the plan. Both installers copy the addon to `<game>/<client>/Interface/AddOns/WowMcp`
(`_forever_` by default; a copy of the old one is kept in a backup folder), install the server, check the `## Interface:` number and run `--check`.
Nothing else is written in the game directory.

| OS | Command | Server ends up in | Backups |
|---|---|---|---|
| Windows | `powershell -ExecutionPolicy Bypass -File .\install.ps1 -WowDir "C:\Program Files (x86)\World of Warcraft"` | `bin\wow-mcp.exe` (the old one is renamed to `.old.exe`) | `backup\` |
| macOS | `./install.sh` (default game directory `/Applications/World of Warcraft`) | `~/.local/share/wow-mcp/wow-mcp` | `~/.local/share/wow-mcp/backup/` |
| Linux (Wine, Lutris, Proton) | `./install.sh --wow-dir "$HOME/Games/battlenet/drive_c/Program Files (x86)/World of Warcraft"` | `~/.local/share/wow-mcp/wow-mcp` | `~/.local/share/wow-mcp/backup/` |

`install.sh` also accepts `--client`, `--install-dir`, `--keep`, `--skip-check` and `--rollback` (see the README). Then add the server to your MCP
client with the snippet the installer prints. On the first installation `--check` may report a missing dump: this is normal until you run `/mcp reload`.

## 2. Restart and first dump

1. Close your MCP client (for example your MCP client, including its tray or menu bar icon) completely and start it again.
2. In the game: `/reload` (the addon loads). Without an earlier dump, `/mcp status` will say there is no dump for this character.
3. `/mcp note test`: the addon confirms the note was saved (`/mcp note` without text removes it).
4. `/mcp reload`: dump and write to disk. Chat shows a line with the number of quests and section errors.
5. `/mcp status`: version 0.8.0, dump time, a "Dump collected in … ms" line and no errors. Above 200 ms there is a
   warning with the slowest sections (then consider `/mcp off <section>`).
6. In your MCP client ask for `wow_status`. It should show `serverVersion` 0.10.0, `addonVersion` 0.8.0 for the character, a fresh `dumpedAt`,
   `dumpTimeMs`, the note `test` and an empty or short `alerts`.
7. In `wow_character` check `resting`, `bindLocation` and `zone`; also ask for `wow_search` (e.g. the name of an item from your bags).

## 3. Verifying auctions, dungeons and talents (roadmap 4-6)

These sections are collected only after you open a window in the game. For each one: open the window, and finally `/mcp reload`.

| What | Do in the game | Ask your assistant | What to look for |
|---|---|---|---|
| Auctions | open the auction house, search for any item | `wow_auctions` | whether prices are present; `source` = `list` (classic API) or `browse`/`commodity`/`item` (C_AuctionHouse) |
| Dungeons | open the LFG window (group finder) | `wow_dungeons` | whether level ranges come from the client; if not, the server uses the classic table |
| Talents | open the talents window | `wow_talents` | whether nodes have a `description` (text from `C_Spell.GetSpellDescription`) |
| Dungeon loot | `/mcp loot deadmines` (or `/mcp loot` inside a dungeon) | `wow_dungeon_loot` | whether chat says what was read; if it says the client has no Encounter Journal for addons, do `/mcp probe` and check `probe.globals` (`EJ_*`) and `probe.addons` |
| Legacy | open the Legacy window (Y) | `wow_legacy` | whether a perk tree with Professions / Adventure / Resourcefulness was found; if not, `discovery=true` shows the client's Legacy API names |

Note what worked and what came back empty: the roadmap fix for each section is based on this. Also check whether `/mcp probe`
saves `probe.events` (which events the client accepted).

## 3a. What to check in the game after updating to 0.6.0 / 0.7.0

Go through them in order, finishing each item with `/mcp reload`, then ask your assistant. Note what came back empty.

1. **Quest rewards.** `wow_quest_rewards` without arguments: whether the quests in the log have `choices` with different items. If
   several quests have an identical list (`suspect: true`), the rewards were read by the wrong ID and the server does not recommend a choice; report it.
2. **Quest windows.** Talk to an NPC who offers a quest (do not accept it if you do not want it), and turn in one quest with a reward
   choice (before clicking the reward). `wow_section` with `questOffer` and `questTurnIn` should show the title, NPC and rewards;
   `wow_quest_rewards` puts them at the top of the list.
3. **Stable.** Open the stable window; `wow_pet` should show `stable` (name, level, family). Also check `rankReview` and `trainingPlan`
   after opening the Beast Training window (regarding the meaning of `used`: whether the pet already has that rank).
4. **Loot.** Kill something, pick up an item, `/mcp reload`: `wow_changes` and `wow_journal` should show `loot`, and `wow_section` with `loot`
   has the item quality (without it the server writes `unknownQuality`).
5. **AH price in the tooltip.** After searching in the auction house, hover over that item: there should be an "AH: … (age)" line. `/mcp ahtip off`
   hides it, `/mcp ahtip on` restores it.
   **Options page (0.11.0).** `/mcp options` should open the WowMcp page (also under Game Menu, Options, AddOns). It lists the same switches
   as the commands; clicking a checkbox must match `/mcp status` / the matching `/mcp` command. If the client has no usable options API the command says so and
   `/mcp` keeps working.
6. **Multiple NPCs.** Open two trainers and two vendors; `wow_trainer` and `wow_vendor` with the `npc` argument should return each one,
   and `known` should list them. `wow_shopping_list` collects offers from all remembered NPCs.
7. **Talents.** `wow_talents`: whether nodes with rank 0 have a description (`descriptionSource = "tooltip"` is fine), and
   `talents.missingDescriptions` drops after the second dump. Rows and columns should start at 1.
8. **Auctions, bank, mail, recipes.** Open each window and check `wow_auctions`, `wow_bank`, `wow_mail`, `wow_recipes`
   (as in item 3; these are still open roadmap items).
9. **Goals.** `wow_goal_set` with `type=level` and `target=` one level higher, then `wow_goals`. The `goals.json` file is created next to
   the `history` directory (not in the game directory).

## 3b. What to check in the game after updating to addon 0.7.0

The API names for the new sections are candidates that nobody has run in Forever yet. If something comes back empty, do `/mcp probe`,
`/mcp reload` and ask your assistant for the `probe` section (`wow_section`): the list shows which functions and events the client has.
Open the friends and guild windows yourself: the addon only reads what the client has in memory and **does not ask** the game server for these lists.

1. **Social window and guild window.** Open the friends list (and then the guild window, if the character is in a guild), do `/mcp reload`
   and ask for `wow_social`: friends (`friends`) and online guild members with level, class and zone. If the friends window was not read, the
   response contains `friendsHint`, and when the list is empty, `note`; a character without a guild has `inGuild: false` and a sentence saying it is not in a guild; `wow_section` with `friends` and `guild` shows the raw data.
2. **Buyback list.** Sell one thing to a vendor (preferably a cheap one), leave the window open and do `/mcp reload`. `wow_vendor` should have a
   `buyback` field with the price and the age of the record. This is an assumption: after logout the client loses the buyback list.
3. **XP by source.** Kill a few creatures, turn in a quest, `/mcp reload`: `wow_changes` and `wow_journal` should show `xpSources`
   (kills, quests, other). If quest XP is counted twice or nothing is collected, report it along with the chat line.
4. **Equipment sets and PvP.** If you have a set in the equipment manager, `wow_equipment` should show `equipmentSets` with `isEquipped`;
   `wow_character` should show `pvp` (a missing section is not an error when the client lacks these functions).
5. **Aura timers and flight zones.** `wow_buffs` should show the time remaining for buffs with a duration (`expiresAt`, `duration`). Open the flight
   map: the nodes in `wow_section` with `flightPaths` should have `zone`, and `wow_flights` and `wow_quest_route` should use it
   (`matchedBy` = `zone field`). While at it, check what the `unreachable` state means.
6. **Minimap button and compact mode.** A left click on the button should show or hide the panel (no save and no reload);
   `/mcp minimap off` hides it. `/mcp compact on` collapses the panel to a single line with the Save button.
7. **History size.** After a few days, older dumps in `history/<character>` in the server's install folder (`bin\history\<character>` on Windows) should be `.json.gz` files (`--history-compress-days`),
   and the character's directory should not exceed `--history-max-mb`.

## 3c. What to check in the game after updating to addon 0.7.1

1. **Exiting the game.** Do `/mcp reload`, then exit the game completely (not `/reload`) and log in again. `/mcp status` may say
   that at logout the client no longer had the character data and the previous dump was kept; `wow_section` with `character` should show a normal
   character (not level 0, not empty bags), and `wow_status` should have no alert about an empty dump. No file with an empty character should
   be added to `history/<character>` in the server's install folder (`bin\history\<character>` on Windows) (if one is added, the server skips it anyway and reports `skippedUnloadedSnapshots`).
2. **Vendors and trainers.** Visit two vendors and a trainer, `/mcp reload`: `wow_section` with `vendors` and `trainers` should have
   entries by NPC name, and `buyback` should be present after selling something. Event handler errors appear in `addonErrors` as `event …`.
3. **Loot and XP.** Kill a few creatures, loot them, `/mcp reload`. `wow_section` with `loot` and `xpGains`: the sections should always be present (since
   login), and `messagesSeen` / `messagesParsed` say how many of your own messages arrived and how many were recognized. `messagesSecret`
   greater than zero means the client hides the chat content; `xpGains.barXp` is the gain from the XP bar, independent of chat.
4. **Flight zones.** Open the flight map, `/mcp reload`: `flightPaths.zoneSources` says where the zones came from, `nodeFields` shows
   the client's node fields. Report both if `zone` is still missing.
5. **Beast Training.** Open the Beast Training window, `/mcp reload`: in Forever it is saved as `trainer` without `npc`. Check whether the cost
   (`costCopper`) is the pet's training points (`wow_pet` assumes it is).
6. **Recipes, stable, auctions.** Open the professions window (ingredients in `wow_recipes`), the stable (`active` on the active pet)
   and search for something at the auction house (`wow_auctions`).

## 4. When something does not work

General diagnostic mode: in the game `/mcp probe`, then `/mcp reload` (probe attaches the list of the client's API functions and the events the client
accepted). In your MCP client ask for the `probe` section via `wow_section` and the `addonErrors` fields, and describe what is missing. Paste the result
together with the message from `/mcp status` into the conversation with your assistant (or into the developer chat).

The most common problems (for addon 0.7.1 and server 0.9.1):

| Symptom | Cause and what to do |
|---|---|
| A new addon file did not load (new section, command, change in `.toc`) | `/reload` does not re-read the `.toc` file list. After an installation that adds or removes addon files, close the game completely and start it again. A plain `/reload` is enough only when the contents of already loaded files change. |
| After installing, the MCP client still uses the old server, or there are no `wow_*` tools | Your MCP client keeps the server running. Close it completely, including the tray or menu bar icon (closing the window is not enough), and start it again. Also check that the client configuration points to the installed server binary and to your game directory (the `--wow-dir` flag or the `WOW_DIR` variable). |
| `wow_status` contains `addonUpdate`: "Dump from addon X; the server supports 0.7.1 (...). Copy the new addon to AddOns and do /reload." | The dump comes from an older addon than the one the server expects (`expectedAddonVersion`). Run `install.ps1` (it copies the addon), then in the game `/reload` and `/mcp reload`; if old files were open in the game, restart the game. The installed addon version is shown by `/mcp status`. |
| Talent descriptions (`description`) are empty and `talents.missingDescriptions` is greater than zero | The client returns a spell description only after its data is loaded. Open the talents window (the addon then requests the data), wait a moment and do a second `/mcp reload`. Descriptions taken from the tooltip have `descriptionSource = "tooltip"`, which is fine. |
| The sections `trainer`, `vendor`, `buyback`, `bank`, `mail`, `auctions`, `recipes`, `stable`, `questOffer`, `questTurnIn`, `flightPaths`, `friends`, `guild` are empty or old | These sections are saved only after the relevant window is opened in the game, and afterwards are only carried over between dumps. Open the window (for recipes each profession separately, for auctions search for something), and finally `/mcp reload`. The panel highlights "Save" when there is new data from windows. `wow_status` says in the `howToRefresh` field which window to open. |
| The response starts with a line `WARNING: The data is ... old. Before basing advice on it, ask the player to type /mcp reload in game.` | The dump is older than `--stale-minutes` (default 120). The game writes the file only on `/reload`, logout and exit, so type `/mcp reload` and ask again. If that does not help, check `--wow-dir` (`wow-mcp.exe --wow-dir "..." --check`). |
| `addonErrors` in the response or the alert "Addon reported errors while collecting sections (count: N, field addonErrors)"; in the game after a dump, red `error ...` lines | One section failed (entries have the form `section: message`), the rest of the dump is fine. Copy the messages into the conversation with your assistant, do `/mcp probe` and `/mcp reload`. Until it is fixed you can turn the section off: `/mcp off <section>` (except `character`), and turn it on again with `/mcp on <section>`. |
| In `wow_quest_rewards` quests have `suspect: true` and there is no recommendation | At least two quests in the log have an identical list of choice rewards, so the client may have returned one quest's rewards for all of them. The server then does not recommend a choice. Open the quest window at the NPC or the turn-in window (the `questOffer` and `questTurnIn` sections are more reliable), do `/mcp reload` and report the case. |
| Slow dump warning: "The addon dump took ... ms (over 200 ms); the game may stutter briefly on /mcp reload" (in the game: "The dump is heavy ...") | Collection took longer than 200 ms. The slowest sections are listed in the message and in `/mcp status`. Turn off the ones you do not need: `/mcp off <section>`. |
| The addon is marked "out of date" | In the game tick "Load out of date AddOns" or fix `## Interface:` in the `.toc` (the installer warns about this in the Interface check step). |

## 4a. When a new version causes trouble: rolling back

Before you start looking for the cause, you can return to the previous version (chapter 5). The installer restores the last addon copy from
its backup folder and the newest `wow-mcp-<version>.old` (`.old.exe` on Windows) as the server. If the script cannot work (on Windows
e.g. because the MCP client is holding the file), close your MCP client completely and manually rename the current server binary, and name the chosen
`*.old` file `wow-mcp` (`wow-mcp.exe`).

## 5. Rolling back to the previous version

```sh
./install.sh --rollback                                     # macOS, Linux
```

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1 -Rollback   # Windows
```

It restores the last addon copy from the backup folder and the newest old server binary (the current versions also go to the
backup, so a second rollback returns to the newer one). Then restart the MCP client and do `/reload` in the game.
