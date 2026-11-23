// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package flights

import (
	"strings"

	"wowmcp/internal/snapshot"
)

type section struct {
	Nodes       []Node
	HasCaptured bool
	CapturedAt  int64
}

func readSection(s snapshot.Snapshot) (section, bool) {
	raw := s.Map("flightPaths")
	list, has := raw["nodes"]
	if !has {
		return section{}, false
	}
	sec := section{Nodes: ParseNodes(snapshot.AsList(list))}
	sec.CapturedAt, sec.HasCaptured = snapshot.AsInt(raw["capturedAt"])
	return sec, true
}

// ReadNodes reads the known flight points; the second result says whether the dump has a flightPaths section.
func ReadNodes(s snapshot.Snapshot) ([]Node, bool) {
	sec, ok := readSection(s)
	return sec.Nodes, ok
}

// isRetiredNode says whether the point is a retired game data entry (zzOLD prefix, e.g. "zzOLDRiverglades, Farholde Keep"
// next to the real "Farholde Keep, Riverglades" in a Forever 1.60.1 dump).
func isRetiredNode(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), "zzold")
}

// ParseNodes reads the flightPaths.nodes list (skips entries without a name and retired zzOLD entries).
func ParseNodes(list []any) []Node {
	var out []Node
	for _, raw := range list {
		m := snapshot.AsMap(raw)
		name, _ := m["name"].(string)
		if name == "" || isRetiredNode(name) {
			continue
		}
		n := Node{Name: name}
		n.State, _ = m["state"].(string)
		n.Zone, _ = m["zone"].(string)
		n.Continent, _ = m["continent"].(string)
		if isContinentName(n.Zone) {
			n.Continent, n.Zone = n.Zone, ""
		}
		n.NodeID, _ = snapshot.AsInt(m["nodeID"])
		n.ZoneMapID, _ = snapshot.AsInt(m["zoneMapID"])
		out = append(out, n)
	}
	return out
}
