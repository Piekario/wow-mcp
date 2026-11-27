// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package achievements

import (
	"fmt"

	"wowmcp/internal/snapshot"
)

const achievementsCoverage = "The addon lists up to 600 achievements of this character (notListed counts the rest), with the completion date of " +
	"completed ones and, for incomplete ones only, up to 8 criteria (missing ones first). Account-wide, guild and comparison data are not in the dump."

const statisticsCoverage = "Values are the game's own display text from the Statistics tab (up to 400 statistics); statistics the game shows " +
	"as \"--\" (never happened) are left out and only counted in empty."

// AbsentNote explains a missing section: an older addon, or a client without the API.
func AbsentNote(section, api string) string {
	return fmt.Sprintf("The dump has no %s section. Either the WowMcp addon is older than this feature (update it and type /mcp reload), "+
		"or this game client has no %s, so the addon has nothing to read. To tell which: type /mcp probe in game, then /mcp reload, "+
		"and read wow_section with section probe and path globals for the API names. Do not answer from general game knowledge as if it were this character's data.",
		section, api)
}

// View answers wow_achievements.
func View(s snapshot.Snapshot, q Query) (map[string]any, error) {
	section, found := readAchievements(s)
	r, err := Analyse(section, found, q)
	if err != nil {
		return nil, err
	}
	return r.Render(), nil
}

func percent(done, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return snapshot.Round1(float64(done) * 100 / float64(total))
}

func (r Report) Render() map[string]any {
	if !r.Found {
		return map[string]any{"note": AbsentNote("achievements", "achievement API (GetCategoryList, GetAchievementInfo)")}
	}
	out := map[string]any{"view": r.View, "coverage": achievementsCoverage}
	r.renderTotals(out)
	if r.Query.Query != "" || r.Query.Category != "" {
		out["matching"] = map[string]any{"achievements": len(r.Matched), "completed": r.MatchedCompleted}
	}
	switch r.View {
	case ViewSummary:
		out["categories"] = r.renderCategories()
		out["recent"] = r.renderList(false)
	case ViewTimeline:
		r.renderTimeline(out)
	default:
		out["achievements"] = r.renderList(r.View != ViewRecent)
	}
	r.renderListNotes(out)
	return out
}

func (r Report) renderTotals(out map[string]any) {
	sec := r.Section
	if sec.HasPoints {
		out["points"] = sec.Points
	}
	completed, total := sec.Completed, sec.Total
	if !sec.HasCounts {
		total = int64(len(sec.List))
		completed = 0
		for _, a := range sec.List {
			if a.Completed {
				completed++
			}
		}
		out["countsFrom"] = "the achievements listed in the dump (the client gave no totals)"
	}
	out["completed"], out["total"], out["completedPercent"] = completed, total, percent(completed, total)
	if sec.NotListed > 0 {
		out["notListed"] = sec.NotListed
	}
}

func (r Report) renderCategories() []any {
	list := []any{}
	for _, c := range r.Categories {
		entry := map[string]any{"category": r.Catalog.Path(c.ID), "completed": c.Completed, "total": c.Total}
		if c.Total > 0 {
			entry["percent"] = percent(c.Completed, c.Total)
		}
		list = append(list, entry)
	}
	return list
}

func (r Report) renderList(detail bool) []any {
	list := []any{}
	for _, a := range r.Listed {
		list = append(list, r.entry(a, detail))
	}
	return list
}

func (r Report) entry(a Achievement, detail bool) map[string]any {
	out := map[string]any{"name": a.Name, "points": a.Points}
	if path := r.Catalog.Path(a.CategoryID); path != "" {
		out["category"] = path
	}
	if a.Completed {
		out["completed"] = true
		if a.CompletedOn != "" {
			out["completedOn"] = a.CompletedOn
		}
	} else {
		out["completed"] = false
		renderProgress(out, a)
	}
	if detail {
		if a.Description != "" {
			out["description"] = a.Description
		}
		if a.Reward != "" {
			out["reward"] = a.Reward
		}
	}
	return out
}

func renderProgress(out map[string]any, a Achievement) {
	p := a.Progress()
	if !p.Known {
		return
	}
	out["progressPercent"] = snapshot.Round1(p.Fraction * 100)
	if p.Partial {
		out["progressFromCriteria"] = fmt.Sprintf("%d of %d criteria read", a.CriteriaRead, a.CriteriaTotal)
	}
	if a.CriteriaTotal > 1 {
		out["criteria"] = fmt.Sprintf("%d/%d", a.CriteriaDone, a.CriteriaTotal)
	}
	missing, more := a.Missing()
	if len(missing) > 0 {
		out["missing"] = missing
	}
	if more > 0 {
		out["moreMissing"] = more
	}
}

func (r Report) renderTimeline(out map[string]any) {
	out["achievements"] = r.renderList(false)
	if r.Earliest != nil {
		out["earliest"] = map[string]any{"name": r.Earliest.Name, "completedOn": r.Earliest.CompletedOn}
		out["latest"] = map[string]any{"name": r.Latest.Name, "completedOn": r.Latest.CompletedOn}
	}
}

func (r Report) renderListNotes(out map[string]any) {
	if r.Undated > 0 {
		out["undated"] = r.Undated
		addNote(out, "undated counts completed achievements the client gave no date for; they are left out of the dated list.")
	}
	if r.View != ViewSummary && len(r.Listed) < r.ListedTotal {
		out["shown"], out["of"] = len(r.Listed), r.ListedTotal
		which := "the first"
		if r.View == ViewTimeline {
			which = "the latest"
		}
		addNote(out, fmt.Sprintf("Showing %s %d of %d; raise limit to see more.", which, len(r.Listed), r.ListedTotal))
	}
	if r.View == ViewClosest {
		if r.WithoutProgress > 0 {
			out["withoutProgress"] = r.WithoutProgress
			addNote(out, "withoutProgress counts incomplete achievements without criteria in the dump; they cannot be ranked (see view all).")
		}
		if r.Section.CriteriaSkipped > 0 {
			addNote(out, fmt.Sprintf("The addon skipped the criteria of %d incomplete achievements (per-dump budget), so they cannot be ranked.", r.Section.CriteriaSkipped))
		}
		addNote(out, "progressPercent comes from the criteria: the count of a single counted criterion (74/100), otherwise the share of criteria done.")
	}
	if len(r.Listed) == 0 && len(r.Matched) == 0 && (r.Query.Query != "" || r.Query.Category != "") {
		addNote(out, "No achievement in the dump matches query/category.")
	}
}

func addNote(out map[string]any, text string) {
	if prev, ok := out["note"].(string); ok && prev != "" {
		text = prev + " " + text
	}
	out["note"] = text
}

// ViewStatistics answers wow_statistics.
func ViewStatistics(s snapshot.Snapshot, q StatisticsQuery) map[string]any {
	section, found := readStatistics(s)
	return AnalyseStatistics(section, found, q).Render()
}

func (r StatisticsReport) Render() map[string]any {
	if !r.Found {
		return map[string]any{"note": AbsentNote("statistics", "statistics API (GetStatisticsCategoryList, GetStatistic)")}
	}
	categories := []any{}
	for _, c := range r.Categories {
		list := []any{}
		for _, st := range c.List {
			list = append(list, map[string]any{"name": st.Name, "value": st.Value})
		}
		categories = append(categories, map[string]any{"category": c.Name, "statistics": list})
	}
	out := map[string]any{"categories": categories, "shown": r.Shown, "empty": r.Section.Empty, "coverage": statisticsCoverage}
	if r.Section.NotListed > 0 {
		out["notListed"] = r.Section.NotListed
	}
	if r.Shown == 0 {
		out["note"] = "No statistic with a value matches the filters (the game may show it as \"--\", which the dump leaves out)."
	}
	return out
}
