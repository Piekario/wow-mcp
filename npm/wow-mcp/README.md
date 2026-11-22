# wow-mcp

An [MCP](https://modelcontextprotocol.io) server that lets an AI assistant in any MCP client read the state of
your World of Warcraft character: quests, bags, gear, talents, professions, reputation, auctions, progress over time and
more. Everything is read-only: the in-game addon saves the state with the official addon API, and the server reads that
one file. Full documentation: <https://github.com/Piekario/wow-mcp>.

Original author: Piekasz (GitHub: Piekario). License: GPL-3.0-or-later with an author attribution term (see `NOTICE`).

## 1. Install the in-game addon

```sh
npx -y wow-mcp install-addon
```

It finds the game the way Battle.net recorded it (Windows registry), or in the default install folder, and picks the
client folder when there is only one (`_retail_`, `_classic_era_`, `_forever_`, ...). Otherwise tell it:
`npx -y wow-mcp install-addon --wow-dir "D:\Games\World of Warcraft" --client _classic_era_`. It copies the bundled
addon into `<game>/<client>/Interface/AddOns`, moves an older copy aside as a backup and writes nothing else in the game
directory. Restart the game completely, then type `/reload` once in game to write the first dump.

## 2. Add the server to your MCP client

```json
{
  "mcpServers": {
    "wow": {
      "command": "npx",
      "args": ["-y", "wow-mcp"],
      "env": { "WOW_DIR": "C:\\Program Files (x86)\\World of Warcraft" }
    }
  }
}
```

Run `npx -y wow-mcp --check` to see what the server finds, and `npx -y wow-mcp --help` for every option.

npm installs only the prebuilt server for your platform (Windows, macOS and Linux on x64 and arm64), so you need
neither Go nor a separate download. The dump history is kept in your user data directory
(`%LOCALAPPDATA%\wow-mcp`, `~/Library/Application Support/wow-mcp` or `~/.local/share/wow-mcp`), so it survives updates;
pass `--history-dir` to choose another place.
