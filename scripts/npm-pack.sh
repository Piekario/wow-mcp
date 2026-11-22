#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.
# Builds the npm packages under dist/npm/ (esbuild-style: one launcher package plus one package per platform):
#
#   @piekasz/wow-mcp                 bin/wow-mcp.js launcher + the in-game addon (npx -y @piekasz/wow-mcp install-addon)
#   @piekasz/wow-mcp-<os>-<arch>     the prebuilt Go server for that platform (os/cpu fields let npm pick exactly one)
#
# All packages carry the server version from server/main.go. Publish with scripts/npm-publish.sh (or `npm publish`
# in each directory, platform packages first). Environment: GO, NPM_DIR, PLATFORMS.
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
GO=${GO:-go}
out=${NPM_DIR:-$root/dist/npm}
sver=$("$root/scripts/version.sh" server)
[ -n "$sver" ] || { echo "ERROR: cannot read serverVersion from server/main.go" >&2; exit 1; }
# npm scope: the unscoped names were refused by npm (too similar to an existing package, spam filter)
scope=${NPM_SCOPE:-@piekasz}
platforms=${PLATFORMS:-"windows/amd64 windows/arm64 darwin/amd64 darwin/arm64 linux/amd64 linux/arm64"}

# the addon is the only directory under addon/ that holds a <name>/<name>.toc
addon_name=""
for toc in "$root"/addon/*/*.toc; do
	dir=$(basename "$(dirname "$toc")")
	[ "$(basename "$toc" .toc)" = "$dir" ] && addon_name=$dir
done
[ -n "$addon_name" ] || { echo "ERROR: no addon/<name>/<name>.toc found" >&2; exit 1; }

rm -rf "$out"
mkdir -p "$out"
deps=""
echo "wow-mcp npm packages $sver (addon $addon_name)"
for platform in $platforms; do
	goos=${platform%/*}
	goarch=${platform#*/}
	case $goos in windows) os=win32 exe=wow-mcp.exe ;; *) os=$goos exe=wow-mcp ;; esac
	case $goarch in amd64) cpu=x64 ;; *) cpu=$goarch ;; esac
	name="wow-mcp-$os-$cpu"
	pkg="$out/$name"
	echo "==> $name"
	mkdir -p "$pkg/bin"
	(cd "$root/server" && CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch "$GO" build -trimpath -ldflags "-s -w" -o "$pkg/bin/$exe" .)
	chmod 755 "$pkg/bin/$exe"
	cp "$root/LICENSE" "$root/NOTICE" "$pkg/"
	cat >"$pkg/package.json" <<JSON
{
  "name": "$scope/$name",
  "version": "$sver",
  "description": "Prebuilt wow-mcp server for $os/$cpu. Install the $scope/wow-mcp package instead of this one.",
  "repository": { "type": "git", "url": "git+https://github.com/Piekario/wow-mcp.git" },
  "license": "SEE LICENSE IN LICENSE",
  "author": "Piekasz (https://github.com/Piekario)",
  "os": ["$os"],
  "cpu": ["$cpu"],
  "files": ["bin/", "LICENSE", "NOTICE"]
}
JSON
	printf 'This package holds the prebuilt wow-mcp server for %s/%s. Use: npx -y @piekasz/wow-mcp (see https://github.com/Piekario/wow-mcp)\n' "$os" "$cpu" >"$pkg/README.md"
	deps="$deps${deps:+,}\"$scope/$name\": \"$sver\""
done

main="$out/wow-mcp"
mkdir -p "$main/addon"
cp -R "$root/npm/wow-mcp/bin" "$main/bin"
chmod 755 "$main/bin/wow-mcp.js"
cp -R "$root/addon/$addon_name" "$main/addon/$addon_name"
cp "$root/npm/wow-mcp/README.md" "$root/LICENSE" "$root/NOTICE" "$main/"
node -e '
const fs = require("fs");
const [src, dst, version, deps] = process.argv.slice(1);
const pkg = JSON.parse(fs.readFileSync(src, "utf8"));
pkg.version = version;
pkg.optionalDependencies = JSON.parse("{" + deps + "}");
fs.writeFileSync(dst, JSON.stringify(pkg, null, 2) + "\n");
' "$root/npm/wow-mcp/package.json" "$main/package.json" "$sver" "$deps"
echo "==> wow-mcp"
ls "$out"
