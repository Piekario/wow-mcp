# Sharing a dump

wow-mcp reads whatever the game client gives the addon, and that differs between classes, factions, client versions and
languages. Only a Hunter and a Warlock on an English WoW: Forever client have been checked in the real game. A dump from
your character lets every later change be tested against your class too.

Sharing is your decision and your own step. The server **never uploads anything and makes no network connection**; it only
writes a file on your disk.

## How

1. In game, open the windows you use (trainer, vendor, bank, mailbox, auction house, professions, talents), type
   `/mcp probe` and then `/mcp reload`. The probe adds the list of client API functions, the most useful part for a client
   nobody has seen yet.
2. Ask your assistant: *"Share an anonymized dump of my character."* It calls `wow_share_dump`. Without an assistant, run
   the server in a terminal with the same options your MCP client uses and `--share-dump`:

   ```sh
   wow-mcp --wow-dir "<path to World of Warcraft>" --share-dump
   ```

   `--share-character <name>` picks a character other than the most recently dumped one.
3. The answer names the file, for example `shared/wow-mcp-dump-warlock-31-enUS-20261124-161500.json`, next to the server's
   history directory (`--share-dir` changes the place; a path inside the game directory is rejected).
4. **Open the file and read it.** It is plain JSON.
5. Attach it to a new [Share a dump from your class](https://github.com/Piekario/wow-mcp/issues/new?template=share_dump.yml)
   issue. The issue is public.

## From inside the game: the client report

`/mcp share` opens a window with a short text report and the address of the issue form. Select the text, press Ctrl+C
and paste it into the form's "Client report from the game" field. It is the quick way when you cannot run the server
command, and it complements the full dump.

The report is built from a fixed list of fields, not by filtering the dump: addon and schema version, client version,
build, interface number and language, class, race, faction and level, why and how fast the last dump was taken, the
name and size of every section, sections that are missing or disabled, the addon's own error texts, the events the
client rejected and, after `/mcp probe`, how many functions each client API namespace has. It contains no name of a
character, realm, guild or player, no item and no gold. The character's own name is cut out of error texts.

The addon cannot send anything: it shows the text and you copy it. The same holds for the reminder.

**The reminder.** At most once a month, some time after login, the addon may print one chat line offering the window,
with a link that opens it. It appears only for a class, client build and language for which this account has not opened
the window yet, never before the character has a saved dump, and at most three times; opening the window once ends it
for that combination. `/mcp share off`, the "Do not remind me" button in the window or the "Sharing reminder" switch
on the options page turns it off for good; `/mcp share on` turns it back on.

## What is removed or replaced

| In the dump | In the shared file |
|---|---|
| Character name | `Player`, also inside other texts (a quest text that greets the character, a title) |
| Realm name, realm number (`character.realmId`), player GUIDs, the character key | removed |
| Guild name | `Guild 1`; the guild's message of the day and information text are removed |
| Guild members | names become `Player 2`, `Player 3`, ...; member notes are removed |
| Friends | names become aliases, friend notes are removed |
| Group members and inspected players | names become aliases (the same player keeps the same alias everywhere) |
| Mail | senders become `Sender 1`, ... (a sender who is a known player keeps that player's alias); subjects are removed |
| Auction owners, when a client lists them | aliases |
| Your note for the assistant (`/mcp note`) | removed |
| Macros | names become `Macro 1`, ...; bodies are removed |
| Equipment sets | names become `Set 1`, ... |
| Pet names | `Pet 1`, ... unless the pet still carries the name of its family or species |
| Item tooltips | lines in angle brackets other than the game's standard hints are removed, which covers `<Made by Name>` in any language |
| A section this server version has no rules for | left out whole, and listed under `report.sectionsLeftOut` |

Every name collected from the fields above is also searched for, as a whole word and in any letter case, in all remaining
text and keys of the dump. If a known name is still found after that, the server refuses to write the file.

The file starts with a `report`: how many names were replaced, how many texts removed, which sections it holds. The report
contains no original text.

## What stays

- Class, race, faction, level, client version, build and language.
- All game data: gear, bags, bank, spells, talents, professions and recipes, quests, reputations, gold, zone and
  coordinates, cooldowns, flight points, vendors and trainers visited (NPC names), auction prices, play time, dump times.
- The `/mcp probe` result and the addon's own diagnostics (section errors, timings).

These describe a character, not a person, but the combination can still be recognizable to someone who knows you in
game (a rare item, an unusual profession pair). Read the file and decide.

The account name is never part of a dump: it exists only in the path of the SavedVariables file.

## What happens to an accepted dump

A maintainer reads the file again and adds it to `server/testdata/shared/`. From then on `go test` (`TestSharedDumps`)
runs on every change:

- the file must be exactly what the current rules would write (anonymizing it again changes nothing), so a rule added
  later is applied to old files as well;
- every tool is called on it and must answer without an error.

Problems the dump reveals (a section the server reads wrongly for that class or language) are fixed with a unit test or a
golden case built from it. To withdraw a dump you shared, open an issue; it is removed from the test data in the next
change (it stays in the repository history, like anything published).

## For maintainers: adding a dump

```sh
cp wow-mcp-dump-<...>.json server/testdata/shared/
cd server && go test -run 'TestSharedDumps' .
```

A new addon section must be added to `knownSections` in `server/internal/sharedump/rules.go` together with a rule for
every field that holds a player's name or free text, and a case in `sharedump_test.go`. Until then the section is not
shared.
