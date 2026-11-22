# Publishing checklist

What the owner has to do to publish wow-mcp. Nothing here has been done yet: the repository is local only, and none of
these steps can be done by the code itself. Tick the boxes as you go.

## 1. Before the first public commit

- [ ] Read `LICENSE` and `NOTICE` once more (GPL-3.0-or-later plus the author attribution term). If the project matters
      commercially, ask a lawyer to read `NOTICE`.
- [ ] Decide the public names: repository (suggested: `wow-mcp`), addon name on addon sites (`WowMcp`).
- [ ] Check that no private data is in the repository: `server/testdata/real/` contains a real dump of the owner's
      character (character name, realm, account number in no file, but check) and `git log` carries the commit author.
      Either accept that or anonymize the fixture and regenerate the golden files (`go test -run Golden -update .`).
- [ ] Run `make test` and `make dist` on a clean checkout.

## 2. GitHub

- [ ] Create the repository on GitHub and push `main`.
- [ ] Check that the CI workflow (`.github/workflows/ci.yml`) passes on Linux, macOS and Windows. It has never run on
      GitHub, so expect small fixes.
- [ ] Tag a release with the server version, e.g. `git tag v0.12.0 && git push --tags`. The release workflow checks that
      the tag equals `serverVersion`, runs the tests, builds the archives for six platforms and attaches them with
      `SHA256SUMS`.
- [ ] Write the release notes from the matching `CHANGELOG.md` entries.

## 3. Trust and safety for users

- [ ] macOS binaries are not signed or notarized. Either sign them (Apple Developer ID) or keep the `xattr` note in the
      README.
- [ ] Windows binaries are not signed either; SmartScreen may warn. Mention it in the release notes.
- [ ] Keep `docs/TOS_COMPLIANCE.md` linked from the release notes: the addon only reads, the server only reads one file.

## 4. Listing the project

- [ ] MCP server registries and lists (for example the official MCP registry and community lists): submit the server
      with the stdio configuration from the README.
- [ ] Addon sites (CurseForge, Wago, WoWInterface): upload the `WowMcp` folder as an addon. Check each site's
      rules for addons that export data for external tools, and set the supported game versions honestly (WoW: Forever
      verified; other clients tested on mocks only).
- [ ] Add screenshots to the README (roadmap item 99): the in-game panel and an example conversation.

## 5. After publishing

- [ ] Ask the first users of non-English clients to run `/mcp probe` and send the result (roadmap item 104).
- [ ] Turn on GitHub issues and add an issue template that asks for `wow_doctor` output and `/mcp status`.
