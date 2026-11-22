#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.
# Prints the versions the release tooling needs, read from the source of truth (nothing is hard-coded):
#   server: `serverVersion` in server/main.go
#   addon:  `## Version:` in addon/WowMcp/WowMcp.toc
# Usage: scripts/version.sh server|addon
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
case "${1:-}" in
server)
	sed -n 's/^[[:space:]]*serverVersion[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' "$root/server/main.go" | head -n 1
	;;
addon)
	sed -n 's/^##[[:space:]]*Version:[[:space:]]*\([^[:space:]]*\).*/\1/p' "$root/addon/WowMcp/WowMcp.toc" | head -n 1
	;;
*)
	echo "usage: $0 server|addon" >&2
	exit 2
	;;
esac
