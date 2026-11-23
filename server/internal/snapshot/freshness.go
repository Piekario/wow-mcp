// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package snapshot

import (
	"fmt"
	"time"
)

// Clock returns the current server time; tests supply a fixed one.
type Clock func() time.Time

// Freshness is the data freshness policy: the server clock and the threshold after which a snapshot
// (or a section saved when a window was opened) is too old to base advice on.
// It is injected wherever time matters, instead of using global variables.
type Freshness struct {
	Now        Clock
	StaleAfter time.Duration // 0 disables warnings
}

// Age is the age of a Unix moment relative to the clock; never negative.
func (f Freshness) Age(unix int64) time.Duration {
	return max(f.Now().Sub(time.Unix(unix, 0)), 0)
}

// Until says how much time is left until a Unix moment; passed = it has already passed.
func (f Freshness) Until(unix int64) (left time.Duration, passed bool) {
	d := time.Unix(unix, 0).Sub(f.Now())
	return d, d <= 0
}

func (f Freshness) isStale(age time.Duration) bool {
	return f.StaleAfter > 0 && age >= f.StaleAfter
}

// SnapshotMeta describes when the data comes from, so the recipient can see its age.
// When the snapshot is older than StaleAfter, it adds stale=true and a warning.
func (f Freshness) SnapshotMeta(name string, s Snapshot) map[string]any {
	meta := map[string]any{"character": name}
	if text, ok := s["dumpedAtText"].(string); ok {
		meta["dumpedAt"] = text
	}
	if unix, ok := s.DumpedAt(); ok {
		age := f.Age(unix)
		meta["ageMinutes"] = int64(age.Minutes())
		if f.isStale(age) {
			meta["stale"] = true
			meta["warning"] = fmt.Sprintf("The data is %s old. Before basing advice on it, ask the player to type /mcp reload in game.", FormatAge(age))
		}
	}
	if errs, ok := s["errors"].([]any); ok && len(errs) > 0 {
		meta["addonErrors"] = errs
	}
	return meta
}

// CapturedMeta describes the age of a section collected when a window is opened (trainer, vendor, bank, flightPaths...).
// These sections can be much older than the rest of the snapshot, because they hold the state from the last visit.
func (f Freshness) CapturedMeta(section map[string]any) map[string]any {
	return f.CapturedMetaOf(CapturedOf(section))
}

// Captured is when a window section was saved; Known is false for a section without that time.
type Captured struct {
	At    int64
	Known bool
}

func CapturedOf(section map[string]any) Captured {
	at, ok := section["capturedAt"].(int64)
	return Captured{At: at, Known: ok}
}

// CapturedMetaOf is CapturedMeta for a section already read into a type.
func (f Freshness) CapturedMetaOf(c Captured) map[string]any {
	if !c.Known {
		return nil
	}
	age := f.Age(c.At)
	meta := map[string]any{
		"capturedAt":         FormatTime(c.At),
		"capturedAgeMinutes": int64(age.Minutes()),
	}
	if f.isStale(age) {
		meta["capturedNote"] = fmt.Sprintf("This is the state from a visit %s ago; it may have changed. For fresh data: open the window in game and type /mcp reload.", FormatAge(age))
	}
	return meta
}

// Merge copies the meta fields into the response (e.g. the result of CapturedMeta).
func Merge(out, meta map[string]any) {
	for k, v := range meta {
		out[k] = v
	}
}

// IsStale says whether a Unix moment (e.g. a window's capturedAt) is older than the StaleAfter threshold.
func (f Freshness) IsStale(unix int64) bool { return f.isStale(f.Age(unix)) }
