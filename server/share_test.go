// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/mcpserver"
	"wowmcp/internal/sharedump"
	"wowmcp/internal/storage"
)

// TestSharedDumps runs the whole server over every dump players have shared (testdata/shared/): the file must be
// one the current rules would write unchanged, and every tool must answer it.
func TestSharedDumps(t *testing.T) {
	files := fixtures.SharedFiles(t)
	if len(files) == 0 {
		t.Fatal("testdata/shared has no dumps")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			lua, format := fixtures.SharedLua(t, file)
			if format != sharedump.Format {
				t.Fatalf("format %q: not a file written by wow_share_dump", format)
			}
			sharedDumpIsClean(t, file)
			everyToolAnswers(t, lua)
		})
	}
}

// sharedDumpIsClean: anonymizing the accepted file again must change nothing and leave no section out.
func sharedDumpIsClean(t *testing.T, file string) {
	t.Helper()
	snap := fixtures.Shared(t, file)
	again, err := sharedump.Anonymize(fixtures.SharedCharacter, snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Report.LeftOut) > 0 {
		t.Errorf("sections without privacy rules: %v", again.Report.LeftOut)
	}
	if fixtures.JSON(t, again.Snapshot) != fixtures.JSON(t, snap) {
		t.Error("the current rules would still change this file: run it through wow-mcp --share-dump again before committing it")
	}
}

func everyToolAnswers(t *testing.T, lua []byte) {
	t.Helper()
	tmp := t.TempDir()
	dump := filepath.Join(tmp, storage.SavedVariablesFile)
	if err := os.WriteFile(dump, lua, 0o644); err != nil {
		t.Fatal(err)
	}
	history := &storage.History{Dir: filepath.Join(tmp, "history"), Keep: 5}
	server := newServer(&storage.Store{File: dump}, history, fixtures.At(0), filepath.Join(tmp, "goals.json"), withShares(filepath.Join(tmp, "shared")))
	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "shared-dumps", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close(); ss.Close() })

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if needsArguments(tool) {
			continue
		}
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool.Name, Arguments: map[string]any{}})
		if err != nil {
			t.Errorf("%s: %v", tool.Name, err)
		} else if res.IsError && !expectedRefusal[tool.Name] {
			t.Errorf("%s: %v", tool.Name, res.Content[0].(*mcp.TextContent).Text)
		}
	}
}

// expectedRefusal: tools that answer an empty call with an error by design (no goal given, one character only).
var expectedRefusal = map[string]bool{"wow_goal_set": true, "wow_compare_characters": true}

func needsArguments(tool *mcp.Tool) bool {
	schema, _ := tool.InputSchema.(map[string]any)
	required, _ := schema["required"].([]any)
	return len(required) > 0
}

func TestRunShareDump(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "shared")
	var out bytes.Buffer
	if err := runShareDump(&out, &storage.Store{File: fixtures.Testdata("WowMcp_forever.lua")}, "", dir); err != nil {
		t.Fatal(err)
	}
	written, _ := filepath.Glob(filepath.Join(dir, "wow-mcp-dump-hunter-*.json"))
	if len(written) != 1 {
		t.Fatalf("files in the share directory: %v", written)
	}
	data, _ := os.ReadFile(written[0])
	if bytes.Contains(data, []byte("Piekasz")) {
		t.Error("the character name is in the shared file")
	}
	fixtures.WantAll(t, "--share-dump output", out.String(), written[0], "Removed or replaced:", "Kept:", "Nothing was sent anywhere", sharedump.IssueURL)

	if err := runShareDump(&out, &storage.Store{File: fixtures.Testdata("WowMcp_forever.lua")}, "Nobody", dir); err == nil {
		t.Error("an unknown character was accepted")
	}
	if err := runShareDump(&out, &storage.Store{File: fixtures.Testdata("WowMcp_forever.lua")}, "", ""); err == nil {
		t.Error("no share directory was accepted")
	}
}

func TestResolveShareDir(t *testing.T) {
	if got := resolveShareDir("", filepath.Join("/srv", "wow", "history")); got != filepath.Join("/srv", "wow", "shared") {
		t.Errorf("default next to history: %q", got)
	}
	abs := filepath.Join(t.TempDir(), "out")
	if got := resolveShareDir(abs, "/x/history"); got != abs {
		t.Errorf("absolute path: %q", got)
	}
	game := t.TempDir()
	if err := storage.RequireOutside("directory for shared dumps (--share-dir)", filepath.Join(game, "shared"), game); err == nil {
		t.Error("a share directory inside the game directory should be rejected")
	}
}

// Without a share directory the tool says so instead of writing somewhere unexpected.
func TestShareDumpDisabled(t *testing.T) {
	db, err := (&storage.Store{File: fixtures.Testdata("WowMcp_forever.lua")}).Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mcpserver.ShareDump(db, "", "test", nil); err == nil || !strings.Contains(err.Error(), "--share-dir") {
		t.Errorf("error: %v", err)
	}
}
