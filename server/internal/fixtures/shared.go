// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package fixtures

import (
	"path/filepath"
	"sort"
	"testing"

	"wowmcp/internal/savedvars"
	"wowmcp/internal/snapshot"
)

// Anonymized dumps shared by players (wow_share_dump), accepted into testdata/shared/. Every file there is
// checked by TestSharedDumps: dropping a new file in is all it takes to cover another class, client or language.

// SharedCharacter is the key a shared dump is loaded under: the alias the anonymizer gives the character.
const SharedCharacter = "Player"

// SharedFiles lists the shared dumps, as paths relative to testdata.
func SharedFiles(t testing.TB) []string {
	t.Helper()
	found, err := filepath.Glob(Testdata(filepath.Join("shared", "*.json")))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(found)
	files := make([]string, len(found))
	for i, path := range found {
		files[i] = filepath.Join("shared", filepath.Base(path))
	}
	return files
}

// SharedLua returns a WowMcp.lua file holding the dump of one shared file, and the file's format marker.
func SharedLua(t testing.TB, file string) (lua []byte, format string) {
	t.Helper()
	envelope, _ := readJSON(t, file).(map[string]any)
	dump, ok := envelope["snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("%s: no snapshot object; is it a file written by wow_share_dump?", file)
	}
	format, _ = envelope["format"].(string)
	return luaFile(SharedCharacter, dump), format
}

// Shared returns one shared dump as the domain sees it (after the Lua parser).
func Shared(t testing.TB, file string) snapshot.Snapshot {
	t.Helper()
	lua, _ := SharedLua(t, file)
	vars, err := savedvars.Parse(lua)
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	return onlyCharacter(vars, SharedCharacter)
}
