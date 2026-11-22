#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.
# Publishes the packages built by scripts/npm-pack.sh: platform packages first, the wow-mcp launcher last, so the
# launcher never points at a platform version that is not on the registry yet. Needs `npm login` or NODE_AUTH_TOKEN.
# Extra arguments go to every `npm publish` (for example --dry-run or --provenance).
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
out=${NPM_DIR:-$root/dist/npm}
[ -d "$out/wow-mcp" ] || { echo "ERROR: run scripts/npm-pack.sh first" >&2; exit 1; }
for pkg in "$out"/wow-mcp-*; do
	(cd "$pkg" && npm publish --access public "$@")
done
(cd "$out/wow-mcp" && npm publish --access public "$@")
