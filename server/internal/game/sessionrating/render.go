// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package sessionrating

import (
	"fmt"
	"strings"

	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// View rates the character's latest session against the median of earlier ones.
func View(name string, current snapshot.Snapshot, tr progress.Tracker) (map[string]any, error) {
	r, err := rate(name, current, tr)
	if err != nil {
		return nil, err
	}
	return r.Render(), nil
}

func (r Rating) Render() map[string]any {
	out := map[string]any{"assumption": assumptionTxt, "totalSessions": r.Sessions}
	switch {
	case r.Sessions == 0:
		out["note"] = "No sessions in history."
		return out
	case r.Current == nil:
		out["note"] = "The current session is too short (at least 2 dumps and 5 minutes of play are needed), so there is no pace yet. Try again after the next /mcp reload."
		return out
	}
	out["current"] = r.Current.view()
	out["comparedSessions"] = r.Compared
	if !r.Enough {
		out["note"] = fmt.Sprintf("Too few sessions to compare: %d reliable, at least %d needed (the current one and two earlier ones).", r.Compared+1, minSessions)
		return out
	}
	out["median"] = r.Median.view()
	out["ratio"] = map[string]any{"xp": r.XP.render(), "gold": r.Gold.render(), "quests": r.Quests.render()}
	out["verdict"] = r.verdict()
	return out
}

func (p Pace) view() map[string]any {
	out := map[string]any{
		"xpPerHour":     snapshot.Round1(p.XPPerHour),
		"goldPerHour":   snapshot.Money(int64(p.GoldPerHour)).Signed(),
		"questsPerHour": snapshot.Round1(p.QuestsPerHour),
	}
	if p.Hours > 0 {
		out["hours"] = snapshot.Round1(p.Hours)
		out["start"] = snapshot.FormatTime(p.Start)
	}
	return out
}

func (m Metric) render() map[string]any {
	out := map[string]any{"current": snapshot.Round1(m.Current), "median": snapshot.Round1(m.Median)}
	if !m.Known {
		out["note"] = "the median is not positive, so the ratio is not computed"
		return out
	}
	out["ratio"] = round2(m.Ratio)
	out["label"] = string(m.Speed)
	return out
}

func (r Rating) verdict() string {
	faster, slower, same := 0, 0, 0
	var parts []string
	for _, e := range []struct {
		name string
		m    Metric
	}{{"XP", r.XP}, {"gold", r.Gold}, {"quests", r.Quests}} {
		if !e.m.Known {
			continue
		}
		switch e.m.Speed {
		case Faster:
			faster++
		case Slower:
			slower++
		default:
			same++
		}
		parts = append(parts, fmt.Sprintf("%s %s (x%v)", e.name, e.m.Speed, round2(e.m.Ratio)))
	}
	if len(parts) == 0 {
		return "No positive median in earlier sessions, so there is nothing to compare the pace with."
	}
	head := "Pace similar to a typical session"
	switch {
	case faster > slower && faster >= same:
		head = "This session is faster than a typical one"
	case slower > faster && slower >= same:
		head = "This session is slower than a typical one"
	}
	return head + ": " + strings.Join(parts, ", ") + "."
}

func round2(v float64) float64 { return float64(int64(v*100+sign(v)*0.5)) / 100 }

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}
