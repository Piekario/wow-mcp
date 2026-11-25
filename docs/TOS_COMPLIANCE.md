# Terms of Service compliance

This is a description of how the addon and the server behave, written by the project author. **It is not an approval,
opinion or endorsement from Blizzard**, nor legal advice. Anyone with doubts should read the Terms of Service and the addon policy themselves
and compare them with the code. The project is intended for personal use on your own account.

## What the WowMcp addon does

- Reads the state of your own character through the official addon API (e.g. `C_QuestLog`, `C_Container`, `C_Traits`). It sends nothing and
  does not connect to the network.
- Collects data at two moments: on a dump (the `/mcp` command, `/mcp reload`, the "Save" button, logout) and when the
  player opens a window themselves (trainer, vendor, bank, flight map, professions, mail, auction house, Inspect, stable, quest window
  at an NPC, vendor buyback list, social window, guild window). From the auction house it saves only results the player searched for themselves. From Inspect it saves only the gear of the player
  the user clicked themselves. Deaths, the note (`/mcp note`) and your own loot are recorded as they happen.
- Friends and guild (since addon 0.7.0) are read **only from the client's memory**, and only after the player has opened the social
  or guild window themselves (the client then sends `FRIENDLIST_UPDATE` / `GUILD_ROSTER_UPDATE`). The addon **never asks the game server**
  for these lists: it does not call `ShowFriends`, `GuildRoster`, `SendWho` or their equivalents, and an addon test verifies that none of these
  calls is made. It sends nothing to other players. Equipment sets, PvP, aura timers and the buyback list are also only read.
- Your own XP (`xpGains`) is computed by the addon from the chat messages about the player's own experience (`CHAT_MSG_COMBAT_XP_GAIN`) and from the quest
  turn-in event; it does not record other players' messages.
- The minimap button (since 0.7.0) only shows or hides the panel (the same as `/mcp panel`). It saves nothing, does not reload the
  interface and performs no action in the game; `/mcp minimap off` hides it. The panel's compact mode (`/mcp compact on`) is purely a change to how the panel looks.
- Quest windows at an NPC (`QUEST_DETAIL`, `QUEST_COMPLETE`) are only read by the addon: it records the title, objectives and rewards. It does not accept or turn in
  the quest and does not choose a reward. The same applies to the stable window (it reads the list of pets and calls nothing) and to trainers
  and vendors (it remembers up to 10 most recent offers, without buying or learning).
- Quest rewards from the quest log are read by the addon by quest ID. In older clients that cannot do this, it selects the
  log entry for the duration of the read and immediately restores the previous selection; it does not accept, abandon or turn in quests.
- Quest locations and guide steps (since addon 0.14.0) come from **other addons the player has installed**, read inside the game through
  what those addons expose: the quest database QuestieDB (its public read functions: `Get`, `Exists`, `IdsByName`, `GetAllIds`, and the read-only `Enum` race and class masks) and Guidelime (the guide
  text at the moment a guide addon registers it, observed with `hooksecurefunc`, and the per-character progress table). WowMcp only
  reads: it registers no corrections, changes no data of those addons and calls nothing that acts in the game. It ships no quest database and
  no guide text of its own; the dump holds the locations of the player's current quests and the next steps of the player's own guide, and
  stays on the player's computer. The server still reads only the WowMcp dump, never another addon's files.
- With the TomTom addon installed (since addon 0.18.0), the coordinates in `/mcp quest` answers are chat links. Only a click on a link
  passes its map and coordinates to TomTom (`TomTom:AddWaypoint`, the same call Questie makes), which shows a waypoint and its arrow.
  Nothing is set without that click, the character is not moved or steered, and without TomTom the coordinates stay plain text.
- Your own loot is computed by the addon from the chat messages about the player's own loot (`LOOT_ITEM_SELF` and related). It does not record other players' loot.
- `/mcp share` shows a window with one address (the issue form with a client report without names in it) for the player to copy, and at most once a day one chat line
  offers that window (off with `/mcp share off`). The addon has no way to send the report and does not try: the player
  copies it and opens it in a browser. Nothing opens without a click.
- The item tooltip gets one added line with the last saved auction house price (`/mcp ahtip off` turns it off). It is only
  text in the tooltip: the addon searches for and lists nothing, and knows the price only from searches the player ran themselves.
- For talent descriptions the addon asks the client to load the spell data (`C_Spell.RequestLoadSpellData`). This is a request for data to
  read, not an action in the game.
- For the rows of an auction search the player ran, the addon asks the client to load the item data of rows it has not loaded yet
  (`C_Item.RequestLoadItemDataByID`), as the auction window does for the rows it draws. It is an item cache request: the addon never
  sends an auction house query, never searches, scans, bids or buys.
- It writes data only to SavedVariables (`WowMcpDB`), as every addon does. The game saves this file
  itself, on `/reload` and logout.
- `/mcp loot`, typed by the player, loads the game's own Encounter Journal module (`Blizzard_EncounterJournal`, through
  `C_AddOns.LoadAddOn`) when it is not loaded yet, exactly as opening the journal window does; the journal lists nothing before that.
  It never happens on a dump.
- The only "action" in the game is `ReloadUI()`, called after the `/mcp reload` command or a click on the "Save" button. The panel
  highlights the button as a reminder, but never saves or reloads anything by itself.
- The only request to the game besides reading is `RequestTimePlayed` after login (the same as the `/played` command; the line "Total time played"
  appears in chat). It can be turned off: `/mcp played off`.
- The player can turn off collection of any section (`/mcp off <section>`).

## What the wow-mcp server does

- Reads one file: `WowMcp.lua` from SavedVariables (it checks its modification time every 5 seconds to record every save, and
  reads it again only when it changed). It does not touch the game process, its memory, network traffic or the screen.
- Writes only in its own directory: the dump history (`--history-dir`, by default `history` next to the exe file; since 0.9.0 it
  compresses old dumps with gzip and enforces a size limit, only inside that directory), the long history database
  `history.db` (SQLite, in the history directory: one row per dump and the addon's event log), an optional log
  (`--log`) and the goals file (`--goals-file`, by default `goals.json` next to the history). Paths pointing to the game directory are rejected
  at startup.
- The goals file is changed by a single tool, `wow_goal_set`, and only at the player's request (atomic write: temporary file and rename).
  The server only reads the optional `weights.json` next to it; the player writes it themselves.
- `wow_share_dump` (and `--share-dump`) writes an anonymized copy of a dump into `--share-dir` (by default `shared` next to
  the history), only at the player's request. Nothing is uploaded: the player reads the file and attaches it to an issue
  themselves. See [SHARING_DUMPS.md](SHARING_DUMPS.md).
