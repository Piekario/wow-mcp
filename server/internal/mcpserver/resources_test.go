// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package mcpserver

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/storage"
)

func TestParseResourceURI(t *testing.T) {
	c, s, ok := parseResourceURI(resourceURI("Piekasz-Realm", "bags"))
	if !ok || c != "Piekasz-Realm" || s != "bags" {
		t.Errorf("round trip: %q %q %v", c, s, ok)
	}
	if c, _, ok := parseResourceURI("wow://Zoë%20Ångström/bags"); !ok || c != "Zoë Ångström" {
		t.Errorf("encoding: %q %v", c, ok)
	}
	for _, bad := range []string{"wow://index", "wow:///bags", "wow://a/", "wow://a/b/c", "file://a/b", "wow://a%zz/b"} {
		if _, _, ok := parseResourceURI(bad); ok {
			t.Errorf("%q should not parse", bad)
		}
	}
}

func TestResourcesOverMCP(t *testing.T) {
	dir := t.TempDir()
	dump := filepath.Join(dir, storage.SavedVariablesFile)
	writeDump(t, dump, fixtures.Forever(t))
	server := New(Config{Name: "wow-mcp", Version: "test", Snapshots: &storage.Store{File: dump}, Freshness: fixtures.RealTime()})
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	tpls, err := cs.ListResourceTemplates(ctx, nil)
	if err != nil || len(tpls.ResourceTemplates) != 1 || tpls.ResourceTemplates[0].URITemplate != resourceTemplate {
		t.Fatalf("templates: %+v %v", tpls, err)
	}
	read := func(uri string) (string, error) {
		res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
		if err != nil {
			return "", err
		}
		return res.Contents[0].Text, nil
	}

	text, err := read(resourceIndexURI)
	if err != nil {
		t.Fatal(err)
	}
	var index struct {
		Last       string
		Characters []struct {
			Character string
			Last      bool
			Sections  []struct{ Section, URI string }
		}
	}
	if err := json.Unmarshal([]byte(text), &index); err != nil || index.Last == "" || len(index.Characters) == 0 {
		t.Fatalf("index: %v\n%.300s", err, text)
	}
	var bagsURI string
	for _, c := range index.Characters {
		if c.Character == index.Last && !c.Last {
			t.Errorf("the last character lacks the last marker")
		}
		for _, s := range c.Sections {
			if c.Character == index.Last && s.Section == "bags" {
				bagsURI = s.URI
			}
		}
	}
	if bagsURI == "" {
		t.Fatalf("the index has no bags section of the last character:\n%.500s", text)
	}
	body, err := read(bagsURI)
	if err != nil || !strings.Contains(body, `"bags"`) || !json.Valid([]byte(body)) {
		t.Fatalf("bags section: %v\n%.300s", err, body)
	}
	for _, bad := range []string{resourceScheme + index.Last + "/no-such-section", resourceScheme + "Nobody-Nowhere/bags"} {
		if _, err := read(bad); err == nil {
			t.Errorf("%s: expected an error", bad)
		}
	}
}
