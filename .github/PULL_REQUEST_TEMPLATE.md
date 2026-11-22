## What and why

<!-- What does this change do, and which problem does it solve? Link the issue: "Fixes #123". -->

## How it was tested

<!-- make test, a real game client (which one and version), mocks only, ... -->

## Checklist

- [ ] `make test` passes (server tests, addon fixtures, luacheck, shellcheck)
- [ ] Golden files and addon fixtures are regenerated and reviewed, if output changed
- [ ] `CHANGELOG.md` has an entry
- [ ] The addon stays read-only and the server never writes in the game directory ([TOS_COMPLIANCE](../docs/TOS_COMPLIANCE.md))
- [ ] Everything is in English
