// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package achievements

import (
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func analyse(t *testing.T, s snapshot.Snapshot, q Query) Report {
	t.Helper()
	section, found := readAchievements(s)
	r, err := Analyse(section, found, q)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func names(list []Achievement) []string {
	var out []string
	for _, a := range list {
		out = append(out, a.Name)
	}
	return out
}

func TestProgress(t *testing.T) {
	counted := Achievement{CriteriaTotal: 1, CriteriaRead: 1, Criteria: []Criterion{{Quantity: 74, Required: 100, HasQuantity: true}}}
	if p := counted.Progress(); !p.Known || p.Fraction != 0.74 {
		t.Errorf("a single counted criterion: %+v", p)
	}
	steps := Achievement{CriteriaTotal: 40, CriteriaRead: 25, CriteriaDone: 10}
	if p := steps.Progress(); !p.Known || p.Fraction != 0.25 || !p.Partial {
		t.Errorf("share of all criteria, partial read: %+v", p)
	}
	if p := (Achievement{}).Progress(); p.Known {
		t.Errorf("no criteria data: %+v", p)
	}
	missing, more := Achievement{CriteriaTotal: 10, CriteriaDone: 2, Criteria: []Criterion{{Text: "Ashenvale"}, {Text: "Done", Completed: true}}}.Missing()
	if len(missing) != 1 || missing[0] != "Ashenvale" || more != 7 {
		t.Errorf("missing %v, more %d", missing, more)
	}
}

func TestViews(t *testing.T) {
	s := fixtures.Forever(t)
	summary := analyse(t, s, Query{})
	if !summary.Found || summary.View != ViewSummary || len(summary.Listed) != 4 {
		t.Fatalf("summary: the four dated achievements, got %v", names(summary.Listed))
	}
	if summary.Listed[0].Name != "Hunter: Level 25" || summary.Undated != 1 {
		t.Errorf("newest first, the undated one counted: %v, undated %d", names(summary.Listed), summary.Undated)
	}
	timeline := analyse(t, s, Query{View: "timeline"})
	if timeline.Earliest.Name != "Explore Teldrassil" || timeline.Latest.Name != "Hunter: Level 25" {
		t.Errorf("timeline: %v", names(timeline.Listed))
	}
	closest := analyse(t, s, Query{View: "closest"})
	got := names(closest.Listed)
	if len(got) != 4 || got[0] != "100 Quests Completed" || got[1] != "Explore Darkshore" || closest.WithoutProgress != 1 {
		t.Errorf("closest: %v, without progress %d", got, closest.WithoutProgress)
	}
	quests := analyse(t, s, Query{View: "all", Category: "quests"})
	if len(quests.Matched) != 4 {
		t.Errorf("a parent category includes its subcategories: %v", names(quests.Matched))
	}
	if _, err := Analyse(Section{}, false, Query{View: "nope"}); err == nil {
		t.Error("an unknown view is an error")
	}
}

func TestAbsentSectionsExplain(t *testing.T) {
	out, err := View(snapshot.Snapshot{}, Query{})
	if err != nil {
		t.Fatal(err)
	}
	fixtures.WantAll(t, "achievements", fixtures.JSON(t, out), "no achievements section", "/mcp probe")
	fixtures.WantAll(t, "statistics", fixtures.JSON(t, ViewStatistics(snapshot.Snapshot{}, StatisticsQuery{})), "no statistics section", "/mcp probe")
}

func TestStatistics(t *testing.T) {
	section, found := readStatistics(fixtures.Modern(t))
	r := AnalyseStatistics(section, found, StatisticsQuery{Category: "character"})
	if r.Shown != 3 || len(r.Categories) != 2 || r.Categories[1].Name != "Character > Wealth" {
		t.Errorf("Character and its Wealth subcategory: %+v", r.Categories)
	}
	if r.Categories[1].List[0].Value != "5g 20s 3c" || section.Empty != 2 {
		t.Errorf("money value and the empty count: %+v, %d", r.Categories[1].List, section.Empty)
	}
}
