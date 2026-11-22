// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package snapshot

// Safe reads of values from the snapshot. Numbers from the Lua parser are int64 or
// float64 (the same from JSON history), and an empty Lua table is []any, so a section
// described as a map may arrive as a list.

// AsInt converts a number from the snapshot (int64 or float64) to int64. A float is truncated; NaN, infinities and
// floats outside the int64 range are not numbers here (false), since converting them gives an arbitrary value in Go.
func AsInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case float64:
		if n != n || n >= 1<<63 || n < -(1<<63) { // n != n: NaN
			return 0, false
		}
		return int64(n), true
	case int:
		return int64(n), true
	}
	return 0, false
}

// AsFloat converts a number from the snapshot to float64.
func AsFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case int:
		return float64(n), true
	}
	return 0, false
}

// AsList is a safe cast to a list; nil when the value is not one.
func AsList(v any) []any {
	list, _ := v.([]any)
	return list
}

// AsMap is a safe cast to a keyed table; nil when the value is not one.
func AsMap(v any) map[string]any {
	switch m := v.(type) {
	case map[string]any:
		return m
	case Snapshot:
		return m
	}
	return nil
}
