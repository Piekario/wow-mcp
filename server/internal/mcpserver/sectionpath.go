// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package mcpserver

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// maxListedKeys is how many keys an error lists when a path does not exist.
const maxListedKeys = 40

// pickPath follows a dot path (a key of a map, or a 0-based index of a list) into a section value.
// An empty path returns the value itself.
func pickPath(value any, path string) (any, error) {
	if strings.TrimSpace(path) == "" {
		return value, nil
	}
	current := value
	walked := []string{}
	for _, step := range strings.Split(strings.TrimSpace(path), ".") {
		switch node := current.(type) {
		case map[string]any:
			next, ok := node[step]
			if !ok {
				return nil, fmt.Errorf("no key %q at %s. Keys there: %s", step, where(walked), listKeys(node))
			}
			current = next
		case []any:
			i, err := strconv.Atoi(step)
			if err != nil || i < 0 || i >= len(node) {
				return nil, fmt.Errorf("%q is not an index of the list at %s (0 to %d)", step, where(walked), len(node)-1)
			}
			current = node[i]
		default:
			return nil, fmt.Errorf("%s is a single value, not a container, so %q cannot be followed", where(walked), step)
		}
		walked = append(walked, step)
	}
	return current, nil
}

// where names the place a path step failed at.
func where(walked []string) string {
	if len(walked) == 0 {
		return "the section root"
	}
	return strings.Join(walked, ".")
}

func listKeys(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > maxListedKeys {
		return strings.Join(keys[:maxListedKeys], ", ") + fmt.Sprintf(", ... (%d keys)", len(keys))
	}
	return strings.Join(keys, ", ")
}

// limitValue keeps the first limit items of a list or the first limit keys (sorted) of a map and reports how much was cut.
// For a map it also cuts the lists one level down (e.g. dungeons -> activities): a section whose only key holds a long
// list would otherwise come back whole. It returns the value and, when something was cut, a note with the totals
// (lists one level down under "lists": key -> total and shown); limit <= 0 changes nothing.
func limitValue(value any, limit int) (any, map[string]any) {
	if limit <= 0 {
		return value, nil
	}
	switch v := value.(type) {
	case []any:
		if len(v) > limit {
			return v[:limit], map[string]any{"kind": "list", "total": len(v), "shown": limit}
		}
	case map[string]any:
		return limitMap(v, limit)
	}
	return value, nil
}

func limitMap(v map[string]any, limit int) (any, map[string]any) {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var cut map[string]any
	if len(keys) > limit {
		cut = map[string]any{"kind": "map", "total": len(keys), "shown": limit, "nextKeys": nextKeys(keys[limit:])}
		keys = keys[:limit]
	}
	out := make(map[string]any, len(keys))
	lists := map[string]any{}
	for _, k := range keys {
		out[k] = v[k]
		if list, ok := v[k].([]any); ok && len(list) > limit {
			out[k] = list[:limit]
			lists[k] = map[string]any{"total": len(list), "shown": limit}
		}
	}
	if len(lists) == 0 {
		if cut == nil {
			return v, nil
		}
		return out, cut
	}
	if cut == nil {
		cut = map[string]any{"kind": "map", "total": len(v), "shown": len(v)}
	}
	cut["lists"] = lists
	cut["listsNote"] = "lists one level down were cut to the limit too; read one whole with path (e.g. path=" + firstKey(lists) + ")"
	return out, cut
}

func firstKey(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys[0]
}

// nextKeys lists up to 10 of the keys that follow the shown ones, so the model can ask for them with path.
func nextKeys(rest []string) []string {
	if len(rest) > 10 {
		rest = rest[:10]
	}
	return rest
}
