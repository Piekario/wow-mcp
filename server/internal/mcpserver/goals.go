// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"wowmcp/internal/game/goals"
	"wowmcp/internal/snapshot"
)

type goalArgs struct {
	Type      string  `json:"type,omitempty" jsonschema:"goal type: level, gold, reputation, profession, spell, item"`
	Target    float64 `json:"target,omitempty" jsonschema:"numeric target: level (level), amount in gold (gold), reputation value (reputation without standing), rank (profession), item count (item); not needed for spell"`
	Name      string  `json:"name,omitempty" jsonschema:"profession name (profession) or spell name (spell)"`
	Faction   string  `json:"faction,omitempty" jsonschema:"faction name (reputation)"`
	Standing  string  `json:"standing,omitempty" jsonschema:"target reputation standing instead of target, e.g. Honored (reputation)"`
	Item      string  `json:"item,omitempty" jsonschema:"item name (item)"`
	Note      string  `json:"note,omitempty" jsonschema:"optional note for the goal"`
	Character string  `json:"character,omitempty" jsonschema:"character the goal applies to; empty = most recently dumped character"`
	Remove    string  `json:"remove,omitempty" jsonschema:"ID of the goal to remove (e.g. g2); the other fields are then not needed"`
}

func (s *service) goalBook() goals.Book { return goals.Book{Store: s.goalStore()} }

// goalStore returns the goals port, or a stub that returns an error when goals are disabled.
func (s *service) goalStore() goals.Store {
	if s.Goals == nil {
		return disabledGoals{}
	}
	return s.Goals
}

type disabledGoals struct{}

func (disabledGoals) Read() ([]byte, error) {
	return nil, fmt.Errorf("goals are disabled: the server has no goals file set (--goals-file)")
}
func (disabledGoals) Write([]byte) error { return disabledGoals{}.errOff() }
func (disabledGoals) Where() string      { return "(none)" }
func (disabledGoals) errOff() error {
	return fmt.Errorf("goals are disabled: the server has no goals file set (--goals-file)")
}

// goalSetTool is the only tool that writes: it adds or removes a goal in the server's goals file.
func (s *service) goalSetTool() tool {
	return tool{
		name: "wow_goal_set", title: "Add or remove a goal", writes: true, args: argNames[goalArgs](),
		description: "Adds a player goal (type: level, gold, reputation, profession, spell, item; with target, name, faction, standing, item, note, character) or removes one (remove = ID from wow_goals). " +
			"This is the ONLY tool that writes anything, and it writes only the server's goals file (goals.json next to the snapshot history); it changes nothing in the game or in the game directory. " +
			"Examples: type=level target=25; type=gold target=10 (gold); type=reputation faction=Darnassus standing=Revered; type=profession name=Mining target=150; type=spell name=Aimed Shot; type=item item=Linen Cloth target=40.",
		add: func(server *mcp.Server, def *mcp.Tool) {
			mcp.AddTool(server, def, func(ctx context.Context, req *mcp.CallToolRequest, a goalArgs) (*mcp.CallToolResult, any, error) {
				db, err := s.Snapshots.Load()
				if err != nil {
					return nil, nil, err
				}
				out, err := s.setGoal(db, a)
				if err != nil {
					return nil, nil, err
				}
				return textResult(out)
			})
		},
	}
}

func (s *service) setGoal(db *snapshot.Database, a goalArgs) (map[string]any, error) {
	s.goalsMu.Lock()
	defer s.goalsMu.Unlock()
	book := s.goalBook()
	if a.Remove != "" {
		if a.Type != "" {
			return nil, fmt.Errorf("give either remove (delete) or type (new goal), not both")
		}
		g, err := book.Remove(a.Remove)
		if err != nil {
			return nil, err
		}
		return map[string]any{"removed": g, "goal": g.Label(), "file": s.GoalsFile}, nil
	}
	if a.Type == "" {
		return nil, fmt.Errorf("give the type of the new goal (%v) or remove with the ID of the goal to delete", goals.Types)
	}
	name, snap, err := db.Lookup(a.Character)
	if err != nil {
		return nil, err
	}
	g, err := book.Add(goals.Spec{
		Type: goals.Type(a.Type), Target: a.Target, Name: a.Name, Faction: a.Faction, Standing: a.Standing,
		Item: a.Item, Note: a.Note, Character: name,
	})
	if err != nil {
		return nil, err
	}
	all, err := book.All()
	if err != nil {
		return nil, err
	}
	out := map[string]any{"added": g, "goal": g.Label(), "file": s.GoalsFile}
	for _, r := range goals.View(all, goals.Env{Character: name, Snap: snap, History: s.progress})["goals"].([]any) {
		if m := r.(map[string]any); m["id"] == g.ID {
			out["progress"] = m
		}
	}
	return out, nil
}
