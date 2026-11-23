// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package mcpserver

// Safety properties of the MCP adapter: concurrent requests, the response cap on resource
// reads, and resource URIs that can only name dump sections.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/storage"
)

// connectSession starts a server over the configuration and returns a connected client session.
func connectSession(t *testing.T, cfg Config) *mcp.ClientSession {
	t.Helper()
	server := New(cfg)
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

// MCP requests may arrive concurrently: every call parses the dump afresh, records it in the history and (wow_goal_set)
// rewrites the goals file. No goal may be lost, and nothing may race (run with -race).
func TestConcurrentToolCalls(t *testing.T) {
	dir := t.TempDir()
	dump := filepath.Join(dir, storage.SavedVariablesFile)
	writeDump(t, dump, fixtures.Forever(t))
	history := &storage.History{Dir: filepath.Join(dir, "history"), Keep: 10}
	goalsPath := filepath.Join(dir, "goals.json")
	cs := connectSession(t, Config{Name: "wow-mcp", Version: "test", Snapshots: &storage.Store{File: dump, History: history},
		History: history, HistoryDir: history.Dir, Freshness: fixtures.At(0),
		Goals: storage.OwnFile{Path: goalsPath}, GoalsFile: goalsPath})

	const adds = 12
	readers := []string{"wow_status", "wow_character", "wow_bags", "wow_changes", "wow_progress", "wow_goals", "wow_vendor", "wow_search"}
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < adds; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "wow_goal_set", Arguments: map[string]any{"type": "level", "target": 30}})
			if err != nil || res.IsError {
				t.Errorf("wow_goal_set: %v %v", err, res)
			}
		}()
		go func(tool string) {
			defer wg.Done()
			args := map[string]any{}
			if tool == "wow_search" {
				args["query"] = "Linen"
			}
			if _, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args}); err != nil {
				t.Errorf("%s: %v", tool, err)
			}
		}(readers[i%len(readers)])
	}
	wg.Wait()

	data, err := os.ReadFile(goalsPath)
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Goals []struct{ ID string } `json:"goals"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, g := range file.Goals {
		ids[g.ID] = true
	}
	if len(file.Goals) != adds || len(ids) != adds {
		t.Errorf("after %d concurrent adds the file has %d goals with %d distinct IDs", adds, len(file.Goals), len(ids))
	}
}

// The response cap also covers resources: wow://<character>/<section> returns whole sections like wow_section.
func TestMiddlewareCapsResourceReads(t *testing.T) {
	dir := t.TempDir()
	dump := filepath.Join(dir, storage.SavedVariablesFile)
	writeDump(t, dump, fixtures.Forever(t))
	read := func(limitKB int) string {
		t.Helper()
		cs := connectSession(t, Config{Name: "wow-mcp", Version: "test", Snapshots: &storage.Store{File: dump}, Freshness: fixtures.At(0), MaxResponseKB: limitKB})
		res, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: resourceURI("Piekasz", "bags")})
		if err != nil {
			t.Fatal(err)
		}
		return res.Contents[0].Text
	}
	capped := read(1)
	if len(capped) > 1024 || !strings.Contains(capped, `"truncated": true`) || !strings.Contains(capped, "wow_section") {
		t.Errorf("resource not capped: %d bytes\n%.400s", len(capped), capped)
	}
	if full := read(0); strings.Contains(full, `"truncated"`) || len(full) < 2048 {
		t.Errorf("limit 0 must not cap: %d bytes", len(full))
	}
}

// A resource URI reads only a dump section: not a meta field, not a path outside the snapshot.
func TestResourceURIsReadOnlySections(t *testing.T) {
	dir := t.TempDir()
	dump := filepath.Join(dir, storage.SavedVariablesFile)
	writeDump(t, dump, fixtures.Forever(t))
	cs := connectSession(t, Config{Name: "wow-mcp", Version: "test", Snapshots: &storage.Store{File: dump}, Freshness: fixtures.At(0)})
	for _, uri := range []string{
		"wow://Piekasz/dumpedAt", "wow://Piekasz/errors", "wow://Piekasz/..%2F..%2Fgoals.json", "wow://..%2F..%2Fetc/passwd",
		"wow://Piekasz/%2E%2E", "file:///etc/passwd", "wow://Piekasz/bags/items",
	} {
		if res, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: uri}); err == nil {
			t.Errorf("%s: expected an error, got %.200v", uri, res.Contents[0].Text)
		}
	}
}
