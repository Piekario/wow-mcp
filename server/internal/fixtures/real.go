// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package fixtures

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"wowmcp/internal/savedvars"
	"wowmcp/internal/snapshot"
)

// Real dumps from the game (Forever client 1.60.1) in the JSON history format in testdata/real/.
// The helpers turn them into a SavedVariables-style file so tests take exactly the same path
// as the real game file (Lua parser, Store, tools).
const (
	// RealCharacter is the character key in the real file: name-realm with spaces.
	RealCharacter = "Piekasz-CAMWOW EU4 Realm 7 PvE"
	// RealDumpedAt is the time of the real addon 0.7.0 dump (2026-11-15 23:43:01 in the player's local time).
	RealDumpedAt = 1794782581
	// RealUnloadedDumpedAt is the time of the dump taken at game exit (addon 0.5.0, 22:12:09): the client already had empty data.
	RealUnloadedDumpedAt = 1794777129

	// Real0110DumpedAt is the time of the real addon 0.11.0 dump (2026-11-16, a logout 10 s after /mcp reload).
	Real0110DumpedAt = 1794840582
	// Real0110ReloadAt is the time of the /mcp reload save just before it: the dump the server never saw, whose loot
	// and XP the logout dump started from zero again (the reason for the carried periods of addon 0.12.0).
	Real0110ReloadAt = 1794840572

	// realSnapshotFile: /mcp probe (a manual dump and a PLAYER_LOGOUT dump at /reload), addon 0.7.0.
	realSnapshotFile = "real/snapshot_2026-11-15_addon070.json"
	// real0110File: a history snapshot of addon 0.11.0 (without probe); its flight map was captured by addon 0.7.0.
	real0110File = "real/snapshot_2026-11-16_addon0110.json"
	// realUnloadedFile: a PLAYER_LOGOUT dump at a real game exit (empty bags, 0 gold).
	realUnloadedFile = "real/logout_at_exit_2026-11-15_addon050.json"
)

// RealLua returns the contents of a WowMcp.lua file with one character: the real addon 0.7.0 dump.
func RealLua(t testing.TB) []byte {
	t.Helper()
	return realLua(t, realSnapshotFile)
}

// Real returns the real addon 0.7.0 dump as the domain sees it (after the Lua parser).
func Real(t testing.TB) snapshot.Snapshot {
	t.Helper()
	return parseReal(t, RealLua(t))
}

// Real0110 returns the real addon 0.11.0 dump of 2026-11-16 as the domain sees it (after the Lua parser).
func Real0110(t testing.TB) snapshot.Snapshot {
	t.Helper()
	return parseReal(t, realLua(t, real0110File))
}

// RealUnloaded returns the real dump from game exit, when the client no longer had character data.
func RealUnloaded(t testing.TB) snapshot.Snapshot {
	t.Helper()
	return parseReal(t, realLua(t, realUnloadedFile))
}

func realLua(t testing.TB, file string) []byte {
	t.Helper()
	data, err := os.ReadFile(Testdata(file))
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var raw any
	if err := dec.Decode(&raw); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	var b strings.Builder
	b.WriteString("WowMcpDB = ")
	writeLua(&b, map[string]any{
		"last":       RealCharacter,
		"characters": map[string]any{RealCharacter: raw},
	})
	b.WriteString("\n")
	return []byte(b.String())
}

func parseReal(t testing.TB, lua []byte) snapshot.Snapshot {
	t.Helper()
	vars, err := savedvars.Parse(lua)
	if err != nil {
		t.Fatal(err)
	}
	chars := vars["WowMcpDB"].(map[string]any)["characters"].(map[string]any)
	return chars[RealCharacter].(map[string]any)
}

// writeLua writes a JSON value as a Lua table literal (keys sorted, so the result is stable).
func writeLua(b *strings.Builder, v any) {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString("{")
		for _, k := range keys {
			b.WriteString("[" + luaString(k) + "]=")
			writeLua(b, x[k])
			b.WriteString(",")
		}
		b.WriteString("}")
	case []any:
		b.WriteString("{")
		for _, item := range x {
			writeLua(b, item)
			b.WriteString(",")
		}
		b.WriteString("}")
	case string:
		b.WriteString(luaString(x))
	case json.Number:
		b.WriteString(x.String())
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case nil:
		b.WriteString("nil")
	default:
		panic(fmt.Sprintf("fixtures: unsupported type %T", v))
	}
}

// luaString quotes a string the way the parser reads it: \\ \" and \ddd codes for control characters.
func luaString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\\' || c == '"':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c < 32 || c == 127:
			fmt.Fprintf(&b, "\\%03d", c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}
