// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package addon

import "wowmcp/internal/snapshot"

func read(s snapshot.Snapshot) Info {
	info := Info{Disabled: DisabledSections(s)}
	info.Version = Version(s)
	info.TotalMs, info.HasTotal = DumpTimeMs(s)
	timings := s.Map("timings")
	for _, raw := range snapshot.AsList(timings["slowest"]) {
		e := snapshot.AsMap(raw)
		if ms, ok := snapshot.AsFloat(e["ms"]); ok {
			info.Slowest = append(info.Slowest, SectionTiming{Name: e["name"], Ms: ms})
		}
	}
	return info
}

// Version returns the version of the addon that made the dump ("" for a very old addon).
func Version(s snapshot.Snapshot) string {
	if v, _ := s["addonVersion"].(string); v != "" {
		return v
	}
	v, _ := s.Character()["addonVersion"].(string)
	return v
}

// DumpTimeMs is the time to collect the whole dump in milliseconds (field timings.totalMs); false for older addons.
func DumpTimeMs(s snapshot.Snapshot) (float64, bool) {
	return snapshot.AsFloat(s.Map("timings")["totalMs"])
}

// DisabledSections lists the sections disabled in the addon with the /mcp off command.
func DisabledSections(s snapshot.Snapshot) []string {
	var out []string
	for _, raw := range snapshot.AsList(s["disabledSections"]) {
		if name, ok := raw.(string); ok && name != "" {
			out = append(out, name)
		}
	}
	return out
}

func readWindows(s snapshot.Snapshot) []WindowStatus {
	out := make([]WindowStatus, len(windows))
	for i, w := range windows {
		section, present := s[w.section]
		out[i] = WindowStatus{window: w, Present: present}
		if present {
			out[i].Captured = snapshot.CapturedOf(snapshot.AsMap(section))
		}
	}
	return out
}
