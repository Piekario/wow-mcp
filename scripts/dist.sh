#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.
# Cross-compiles the server and assembles one release archive per platform under dist/, plus SHA256SUMS.
#
#   windows/{amd64,arm64}  -> .zip     bin/wow-mcp.new.exe, addon/WowMcp/, install.ps1   (the layout install.ps1 expects)
#   darwin/linux x amd64/arm64 -> .tar.gz   wow-mcp, WowMcp/, install.sh
#   every archive also has README.md, CHANGELOG.md, LICENSE, NOTICE and docs/
#
# Versions come from the source (scripts/version.sh). Environment: GO, DIST_DIR, SOURCE_DATE_EPOCH (reproducible mtimes).
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
GO=${GO:-go}
dist=${DIST_DIR:-$root/dist}
sver=$("$root/scripts/version.sh" server)
aver=$("$root/scripts/version.sh" addon)
[ -n "$sver" ] || { echo "ERROR: cannot read serverVersion from server/main.go" >&2; exit 1; }
[ -n "$aver" ] || { echo "ERROR: cannot read the addon version from the .toc" >&2; exit 1; }
platforms=${PLATFORMS:-"windows/amd64 windows/arm64 darwin/amd64 darwin/arm64 linux/amd64 linux/arm64"}
epoch=${SOURCE_DATE_EPOCH:-$(date +%s)}

rm -rf "$dist"
mkdir -p "$dist"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT INT TERM

if tar --version 2>/dev/null | grep -q GNU; then
	tar_flags="--sort=name --owner=0 --group=0 --numeric-owner --mtime=@$epoch"
else
	tar_flags=""
fi

echo "wow-mcp server $sver, addon $aver"
for platform in $platforms; do
	goos=${platform%/*}
	goarch=${platform#*/}
	name="wow-mcp-v$sver-addon-$aver-$goos-$goarch"
	stage="$work/$name"
	mkdir -p "$stage/docs"

	exe=wow-mcp
	[ "$goos" = windows ] && exe=wow-mcp.exe
	echo "==> $name"
	(cd "$root/server" && CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch "$GO" build -trimpath -ldflags "-s -w" -o "$work/$exe.$goos-$goarch" .)

	if [ "$goos" = windows ]; then
		mkdir -p "$stage/bin" "$stage/addon"
		cp "$work/$exe.$goos-$goarch" "$stage/bin/wow-mcp.new.exe"
		cp -R "$root/addon/WowMcp" "$stage/addon/WowMcp"
		cp "$root/install.ps1" "$stage/install.ps1"
	else
		cp "$work/$exe.$goos-$goarch" "$stage/wow-mcp"
		chmod 755 "$stage/wow-mcp"
		cp -R "$root/addon/WowMcp" "$stage/WowMcp"
		cp "$root/install.sh" "$stage/install.sh"
		chmod 755 "$stage/install.sh"
	fi
	rm -f "$work/$exe.$goos-$goarch"
	cp "$root/README.md" "$root/CHANGELOG.md" "$root/LICENSE" "$root/NOTICE" "$stage/"
	[ -f "$root/LICENSE" ] && cp "$root/LICENSE" "$stage/"
	cp "$root"/docs/*.md "$stage/docs/"
	find "$stage" -exec touch -d "@$epoch" {} + 2>/dev/null || true

	if [ "$goos" = windows ]; then
		(cd "$work" && zip -q -X -r "$dist/$name.zip" "$name")
	else
		# shellcheck disable=SC2086
		(cd "$work" && tar $tar_flags -czf "$dist/$name.tar.gz" "$name")
	fi
	rm -rf "$stage"
done

cd "$dist"
if command -v sha256sum >/dev/null 2>&1; then
	sha256sum -- *.zip *.tar.gz >SHA256SUMS
else
	shasum -a 256 -- *.zip *.tar.gz >SHA256SUMS
fi
echo "==> dist/"
ls -l "$dist"
