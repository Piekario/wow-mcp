// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"wowmcp/internal/sharedump"
	"wowmcp/internal/snapshot"
)

// Shares keeps the anonymized dumps the player asked for: one file per call, in the server's own directory.
type Shares interface {
	Save(name string, data []byte) (path string, err error)
}

// shareDumpTool writes an anonymized copy of one character's dump for the player to review and publish themselves.
func (s *service) shareDumpTool() tool {
	return tool{
		name: "wow_share_dump", title: "Share an anonymized dump", writes: true, annotate: ownFileAdder, args: argNames[characterArgs](),
		description: "Writes an anonymized copy of the character's latest dump to a file in the server's own directory, so the player can share it with the wow-mcp project and help it support their class, client version and language. " +
			"Names of the character, realm, guild, guild members, friends, group members and mail senders are replaced or removed, together with the note, macro bodies and mail subjects; class, race, level, client version, language, all game data and the /mcp probe result stay. " +
			"NOTHING is sent anywhere: the server makes no network connection. The answer gives the file path, what was removed and kept, and the address of the issue form the player can attach the file to after reading it. " +
			"Call it only when the player asks to share or export a dump; it changes nothing in the game or in the game directory.",
		add: func(server *mcp.Server, def *mcp.Tool) {
			mcp.AddTool(server, def, func(ctx context.Context, req *mcp.CallToolRequest, a characterArgs) (*mcp.CallToolResult, any, error) {
				db, err := s.loadDatabase()
				if err != nil {
					return nil, nil, err
				}
				out, err := ShareDump(db, a.Character, s.Version, s.Shares)
				if err != nil {
					return nil, nil, err
				}
				return textResult(out)
			})
		},
	}
}

// ShareDump anonymizes the dump of one character (empty name = most recently dumped) and saves it. Shared by the
// tool and the --share-dump command.
func ShareDump(db *snapshot.Database, character, serverVersion string, shares Shares) (map[string]any, error) {
	if shares == nil {
		return nil, fmt.Errorf("sharing is disabled: the server has no directory for shared dumps (--share-dir)")
	}
	key, snap, err := db.Lookup(character)
	if err != nil {
		return nil, err
	}
	shared, name, data, err := sharedump.Export(key, snap, serverVersion)
	if err != nil {
		return nil, err
	}
	path, err := shares.Save(name, data)
	if err != nil {
		return nil, fmt.Errorf("cannot write the shared dump: %w", err)
	}
	return sharedump.Answer(shared, path), nil
}
