# Changelog

The format is modeled on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). The WowMcp addon and the wow-mcp server have
separate version numbers. In a real Forever 1.60.1 client the following were run: addon 0.5.0 with server 0.6.0 (2026-11-15, no section
errors, dump 18 ms), addon 0.7.0 with server 0.9.0 (2026-11-15, no section errors, dump 29 ms) and addon 0.11.0 with server 0.12.2
(2026-11-16, no section errors, dump 26.1 ms; `testdata/real/snapshot_2026-11-16_addon0110.json`). The changes of the versions in
between are part of 0.11.0 / 0.12.2 and were run with them. Addon 0.14.0 was run on 2026-11-22 (client build 71037, no section errors, dump 25.7 ms: `questDb` for 22 log quests, `guide` reading TUGs, `/mcp quest The Glowing Shard`). **Server 0.15.0 and the 0.12.0 / 0.13.0 addon changes (carried periods, flight zones, auction gear details) have not yet been checked in a real client**: their fixes are
based on the real dumps (`testdata/real/`), API mocks matching the `/mcp probe` result and golden response tests.

## Unreleased

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
  - Nothing is uploaded: the server still opens no network connection (now checked by a test). The player reads the
    file and attaches it to the new issue form "Share a dump from your class". `wow_status` shows `shareDir`.
  - Accepted dumps live in `server/testdata/shared/`; `TestSharedDumps` requires each to be exactly what the current
    rules would write and calls every tool on it. The first two are the project's own real Hunter dumps.
  - [docs/SHARING_DUMPS.md](docs/SHARING_DUMPS.md): what is removed, what stays, and what happens to an accepted dump.
