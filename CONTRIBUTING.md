# Contributing to wow-mcp

Thank you for helping. Bug reports, ideas, tests on game clients we have not seen, translations of addon messages and
code are all welcome. This file says how to send a change so it can be merged quickly.

## Ground rules

- **Read-only, always.** The addon only reads game state through the official addon API and never automates gameplay.
  The server only reads the SavedVariables file and never writes inside the game directory. Changes that break these
  rules cannot be accepted, see [docs/TOS_COMPLIANCE.md](docs/TOS_COMPLIANCE.md).
- **English everywhere:** code, comments, tool descriptions and outputs, addon messages, documentation, commit messages.
- **Vendor-neutral:** the project works with any MCP client; do not tie features or docs to one product.
- Be kind. This project follows the [Code of Conduct](CODE_OF_CONDUCT.md).

## Reporting a bug or asking for a feature

Open an [issue](https://github.com/Piekario/wow-mcp/issues/new/choose) and pick a template. For a bug, include the
output of the `wow_doctor` tool and of `/mcp status` in game, your game client and version, and your operating system.
Questions and ideas that are not yet a concrete request fit better in
[Discussions](https://github.com/Piekario/wow-mcp/discussions). Security problems go through a private report, see
[SECURITY.md](SECURITY.md).

## Sending a change

1. Fork the repository and create a branch from `main`. Name it after the change (`fix/flight-zone-names`,
   `refactor/domain-types`), not after a tool or a generated ID.
2. Make the change. Keep it focused: one fix or feature per pull request.
3. Run the checks (Go 1.25+, Python 3 with `pip install lupa`, `luacheck`, `shellcheck`):

   ```sh
   make test
   ```

   - Addon behavior changed? Regenerate the addon fixtures (`python3 addon/tests/run_addon.py <mode> server/testdata/<file>`,
     see `scripts/check-addon-fixtures.sh`) and commit them with the code.
   - Tool or prompt output changed? Refresh the golden files with `cd server && go test -run Golden -update .` and review
     the diff. A new tool, prompt or argument needs a golden case (the test fails without one).
4. Add an entry to [CHANGELOG.md](CHANGELOG.md) under the next version.
5. Open a pull request against `main` and fill in the template. CI must pass on Linux, macOS and Windows before merging.
   `main` is protected: changes from contributors go through a pull request.

## Code style

The rules below are the result of a design review. `server/architecture_test.go` checks the ones marked *(checked)* on
every test run, against baselines in `server/testdata/architecture/` that may only go down.

### Server (Go)

- **Layers.** Domain packages `server/internal/game/<context>` import only `snapshot` and other `game/*` packages: no MCP
  SDK, no file system, no network, no logging, no mutable package state *(checked)*. The clock and other outside facts
  come in as arguments (`snapshot.Freshness`) or through small interfaces declared by the package that needs them.
  `storage` does the I/O, `mcpserver` adapts to MCP, `main.go` wires everything.
- **Typed domain.** The dump is an untyped `snapshot.Snapshot` only at the boundary. Inside a context, turn dump values
  into the package's own types in `read.go` (for example a `Recipe` or `Quest` struct) and write the rules against those
  types, not against `map[string]any` and string keys. New code adds no untyped maps outside `read.go` and `render.go`
  *(checked, per package)*.
- **Rules apart from presentation.** A domain function returns a typed result (what was decided: counts, flags, an advice
  constant). Building the answer for the client, with its JSON keys, formatted money and sentences, is a separate step
  in `render.go`. Tests of a rule assert on the typed result; the golden tests cover the rendered answer.
- **No hidden side effects.** A function that reads does not write. The dump history is recorded by one explicit step in
  `mcpserver` after the dump is loaded, not inside `storage.Store.Load`.
- **One place for shared knowledge.** A lookup that more than one package needs (quality names, item classes, money and
  time formatting, rounding) lives in one package and is imported, never copied. Check `itemkind`, `snapshot` and `items`
  before adding a table.
- **Small functions.** At most 80 lines per function *(checked)*. A long function is usually a rule and its rendering
  in one place.
- **Patterns only where they remove code:** a registry for things added often (tools, prompts, alert checks), small
  consumer-side interfaces for I/O, functional options for optional settings. No pattern for one case.
- A new tool is a function in its context, an entry in the matching group in `mcpserver/tools.go` and cases in
  `golden_test.go`. Unit tests live next to the code. Run `gofmt` and `go vet`.

### Addon (Lua 5.1)

- One dump section per file in `addon/WowMcp/Sections/`, registered in the section registry and listed in `WowMcp.toc`.
  Foundation files depend only on files above them in the `.toc`.
- One responsibility per file and at most 350 lines *(checked)*: when a file collects a second job (a guard, a cache, a
  discovery scan), that job gets its own file.
- Only `Store.lua` touches the saved variable; everything else goes through its functions. No other globals.
- Use the `Compat` helpers (`plain`, `callFirst`, `pick`) for client differences, and never write `cond and a or b` when
  `a` may be `nil`. A section must not fail the whole dump: errors are reported per section.

### Both

- Comments only where they explain something the code cannot say (a reason, a client quirk, a non-obvious constraint).
  Do not restate what the code does, and do not leave commented-out code.
- Names come from the game and the player's vocabulary (bags, trainer, rested XP), not from the implementation.
- Keep the layout described in the README's [Structure](README.md#structure) section.

## License of contributions

wow-mcp is licensed under [GPL-3.0-or-later](LICENSE) with an additional term under section 7(b) that requires keeping
the original author attribution (see [NOTICE](NOTICE)). By submitting a contribution you agree that it is licensed under
the same terms (inbound = outbound). You keep the copyright of your contribution; you may add your own copyright line to
files you change substantially, but do not remove or alter the existing copyright and attribution lines.
