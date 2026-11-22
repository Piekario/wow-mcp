#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.
# Publishes the packages built by scripts/npm-pack.sh: platform packages first, the wow-mcp launcher last, so the
# launcher never points at a platform version that is not on the registry yet. A package version that is already on
# the registry is skipped, so a release can be re-run after a partial publish.
# Auth: npm trusted publishing (in GitHub Actions), `npm login`, or NODE_AUTH_TOKEN.
# Extra arguments go to every `npm publish` (for example --dry-run or --provenance).
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
out=${NPM_DIR:-$root/dist/npm}
[ -d "$out/wow-mcp" ] || { echo "ERROR: run scripts/npm-pack.sh first" >&2; exit 1; }

publish() {
	dir=$1
	shift
	name=$(node -p "require('$dir/package.json').name")
	version=$(node -p "require('$dir/package.json').version")
	if npm view "$name@$version" version >/dev/null 2>&1; then
		echo "==> $name@$version is already published, skipping"
		return
	fi
	echo "==> publishing $name@$version"
	(cd "$dir" && npm publish --access public "$@")
}

for pkg in "$out"/wow-mcp-*; do
	publish "$pkg" "$@"
done
publish "$out/wow-mcp" "$@"
