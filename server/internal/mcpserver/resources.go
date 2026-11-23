// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCP resources: the snapshot as read-only JSON documents.
//
//	wow://index                   list of characters and their section URIs (the most recently dumped character is marked as last)
//	wow://<character>/<section>   one snapshot section (the same as wow_section returns)
//
// The index is a dynamic resource, because the list of sections depends on what the game saved at the last /reload;
// resources are registered once at startup and are not refreshed on every write.

const (
	resourceScheme   = "wow://"
	resourceIndexURI = resourceScheme + "index"
	resourceTemplate = resourceScheme + "{character}/{section}"
	resourceMIME     = "application/json"
)

// resourceURI builds a character section URI (the character name is encoded as a path segment).
func resourceURI(character, section string) string {
	return resourceScheme + url.PathEscape(character) + "/" + url.PathEscape(section)
}

// parseResourceURI splits wow://<character>/<section>; false when the URI does not have this shape.
func parseResourceURI(uri string) (character, section string, ok bool) {
	rest, found := strings.CutPrefix(uri, resourceScheme)
	if !found {
		return "", "", false
	}
	c, s, found := strings.Cut(rest, "/")
	if !found || c == "" || s == "" || strings.Contains(s, "/") {
		return "", "", false
	}
	character, err1 := url.PathUnescape(c)
	section, err2 := url.PathUnescape(s)
	if err1 != nil || err2 != nil {
		return "", "", false
	}
	return character, section, true
}

func (s *service) registerResources(server *mcp.Server) {
	server.AddResource(&mcp.Resource{
		URI: resourceIndexURI, Name: "wow-index", Title: "Snapshot index",
		Description: "Characters in the snapshot and the URIs of their sections (wow://<character>/<section>); last is the most recently dumped character. Read-only.",
		MIMEType:    resourceMIME,
	}, s.readResourceIndex)
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: resourceTemplate, Name: "wow-section", Title: "Character snapshot section",
		Description: "One section of a character snapshot as JSON, e.g. wow://Piekasz-Realm/bags (the same data as wow_section). Read-only.",
		MIMEType:    resourceMIME,
	}, s.readResourceSection)
}

func jsonContents(uri string, value any) (*mcp.ReadResourceResult, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: resourceMIME, Text: string(data)}}}, nil
}

func (s *service) readResourceIndex(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	db, err := s.Snapshots.Load()
	if err != nil {
		return nil, err
	}
	characters := []any{}
	for _, name := range db.CharacterNames() {
		sections := []any{}
		for _, section := range db.Characters[name].SectionNames() {
			sections = append(sections, map[string]any{"section": section, "uri": resourceURI(name, section)})
		}
		characters = append(characters, map[string]any{"character": name, "last": name == db.Last, "sections": sections})
	}
	return jsonContents(req.Params.URI, map[string]any{"last": db.Last, "characters": characters})
}

func (s *service) readResourceSection(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri := req.Params.URI
	character, section, ok := parseResourceURI(uri)
	if !ok {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	db, err := s.Snapshots.Load()
	if err != nil {
		return nil, err
	}
	if _, _, err := db.Lookup(character); err != nil {
		return nil, fmt.Errorf("%w: %v", mcp.ResourceNotFoundError(uri), err)
	}
	out, err := s.section(db, sectionArgs{Section: section, Character: character})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", mcp.ResourceNotFoundError(uri), err)
	}
	return jsonContents(uri, out)
}
