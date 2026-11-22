#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.
# Runs the addon harness (Lua 5.1 + API mocks, needs python3 and `pip install lupa`) in every mode and
# compares the generated dumps with the committed fixtures in server/testdata/, then runs the lifecycle scenario from the
# real game (logout guard, carried data, plain values only). Changes nothing in the repository.
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
py=${PYTHON:-python3}
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT INT TERM
status=0
for pair in modern:WowMcp.lua legacy:WowMcp_legacy.lua forever:WowMcp_forever.lua dede:WowMcp_dede.lua; do
	mode=${pair%%:*}
	file=${pair#*:}
	"$py" "$root/addon/tests/run_addon.py" "$mode" "$out/$file" >/dev/null
	if cmp -s "$out/$file" "$root/server/testdata/$file"; then
		echo "addon fixture ok: $mode -> server/testdata/$file"
	else
		echo "ERROR: server/testdata/$file is out of date for mode '$mode'; regenerate it with:" >&2
		echo "  $py addon/tests/run_addon.py $mode server/testdata/$file" >&2
		status=1
	fi
done
if "$py" "$root/addon/tests/run_addon.py" lifecycle; then :; else status=1; fi
exit $status
