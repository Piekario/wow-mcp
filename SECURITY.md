# Security policy

## Supported versions

Only the latest release of the server and the addon gets fixes. Please update before reporting.

## Reporting a vulnerability

Do **not** open a public issue for a security problem. Report it privately through GitHub:
[Security → Report a vulnerability](https://github.com/Piekario/wow-mcp/security/advisories/new). Include what is
affected (server, addon, installer or npm package), the version, steps to reproduce and the impact you see.

You can expect a first answer within 7 days. Once a fix is released, the advisory is published with credit to the
reporter, unless you prefer to stay anonymous.

## Scope

In scope: the wow-mcp server (for example reading or writing files outside its own directory, path traversal, crashes
caused by a crafted SavedVariables file), the installers (`install.ps1`, `install.sh`, `npx wow-mcp install-addon`) and
the WowMcp addon. Out of scope: the game client, other addons, and MCP clients that run the server.
