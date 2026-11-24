// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package achievements covers the dump's achievements and statistics sections: achievement points, progress per
// category, when each achievement was earned, which incomplete ones are closest to done (from their criteria), and
// the counters the game keeps (the Statistics tab). The addon reads them only when the client has the achievement
// API; nobody has confirmed yet that the Forever client does, so a missing section is an expected answer.
package achievements

import (
	"fmt"
	"sort"
	"strings"
)

// Query is the input of wow_achievements.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	View      string `json:"view,omitempty" jsonschema:"summary (default: points, completed/total, progress per category, the 5 most recent), timeline (completed achievements in date order, earliest and latest), recent (newest completed first), closest (incomplete ones ranked by criteria progress, with what is missing) or all (every achievement in the dump)"`
	Query     string `json:"query,omitempty" jsonschema:"optional fragment of the achievement name (case-insensitive)"`
	Category  string `json:"category,omitempty" jsonschema:"optional fragment of the category name (case-insensitive); a parent category also matches its subcategories"`
	Limit     int    `json:"limit,omitempty" jsonschema:"how many achievements to list (default 10 for recent and closest, 50 for timeline, 100 for all)"`
}

// View names.
const (
	ViewSummary  = "summary"
	ViewTimeline = "timeline"
	ViewRecent   = "recent"
	ViewClosest  = "closest"
	ViewAll      = "all"
)

const summaryRecent = 5

var defaultLimits = map[string]int{ViewRecent: 10, ViewClosest: 10, ViewTimeline: 50, ViewAll: 100}

// Criterion is one step of an achievement (from GetAchievementCriteriaInfo).
type Criterion struct {
	Text               string
	Completed          bool
	Quantity, Required int64
	HasQuantity        bool
}

// Achievement is one entry of the dump's list. CompletedOn is YYYY-MM-DD, empty when the client gave no date.
type Achievement struct {
	ID, Points, CategoryID                    int64
	Name, Description, Reward, CompletedOn    string
	Completed                                 bool
	Criteria                                  []Criterion
	CriteriaTotal, CriteriaRead, CriteriaDone int64
}

// Category is one achievement category; ParentID is 0 for a top-level one.
type Category struct {
	ID, ParentID, Total, Completed int64
	Name                           string
}

// Section is the dump's achievements section.
type Section struct {
	Points, Total, Completed   int64
	HasPoints, HasCounts       bool
	Categories                 []Category
	List                       []Achievement
	NotListed, CriteriaSkipped int64
}

// Progress is how far an incomplete achievement is, from its criteria: the quantity of a single counted criterion
// (74/100 quests), otherwise the share of criteria done. Partial is true when the addon read only some criteria.
type Progress struct {
	Fraction float64
	Known    bool
	Partial  bool
}

func (a Achievement) Progress() Progress {
	if a.Completed {
		return Progress{Fraction: 1, Known: true}
	}
	if a.CriteriaTotal == 1 && len(a.Criteria) == 1 && a.Criteria[0].HasQuantity && a.Criteria[0].Required > 1 {
		c := a.Criteria[0]
		return Progress{Fraction: min(1, max(0, float64(c.Quantity)/float64(c.Required))), Known: true}
	}
	if a.CriteriaTotal <= 0 || a.CriteriaRead <= 0 {
		return Progress{}
	}
	return Progress{Fraction: float64(a.CriteriaDone) / float64(a.CriteriaTotal), Known: true, Partial: a.CriteriaRead < a.CriteriaTotal}
}

// Missing lists the criteria still to do (as far as the dump lists them) and how many more are not listed.
func (a Achievement) Missing() (texts []string, more int64) {
	for _, c := range a.Criteria {
		if c.Completed {
			continue
		}
		text := c.Text
		if c.HasQuantity && c.Required > 1 {
			text = fmt.Sprintf("%s (%d/%d)", c.Text, c.Quantity, c.Required)
		}
		texts = append(texts, text)
	}
	more = a.CriteriaTotal - a.CriteriaDone - int64(len(texts))
	return texts, max(0, more)
}

// Catalog gives category names and the parent chain.
type Catalog map[int64]Category

func newCatalog(categories []Category) Catalog {
	c := Catalog{}
	for _, cat := range categories {
		c[cat.ID] = cat
	}
	return c
}

// Path is "Parent > Category" for a category ID.
func (c Catalog) Path(id int64) string {
	cat, ok := c[id]
	if !ok {
		return ""
	}
	if parent, ok := c[cat.ParentID]; ok && parent.Name != "" {
		return parent.Name + " > " + cat.Name
	}
	return cat.Name
}

func (c Catalog) matches(id int64, fragment string) bool {
	if fragment == "" {
		return true
	}
	return strings.Contains(strings.ToLower(c.Path(id)), strings.ToLower(fragment))
}

func nameMatches(name, fragment string) bool {
	return fragment == "" || strings.Contains(strings.ToLower(name), strings.ToLower(fragment))
}

// Report is the result of wow_achievements.
type Report struct {
	Found            bool
	View             string
	Section          Section
	Catalog          Catalog
	Categories       []Category // after the category filter
	Matched          []Achievement
	Listed           []Achievement // the view's achievements, already ordered and limited
	ListedTotal      int           // how many the view had before the limit
	MatchedCompleted int
	Undated          int // completed achievements without a date (timeline, recent)
	Earliest, Latest *Achievement
	WithoutProgress  int // incomplete achievements with no criteria data (closest)
	Query            Query
}

// Analyse applies the view and the filters to the section.
func Analyse(section Section, found bool, q Query) (Report, error) {
	view := strings.ToLower(strings.TrimSpace(q.View))
	if view == "" {
		view = ViewSummary
	}
	if _, ok := defaultLimits[view]; !ok && view != ViewSummary {
		return Report{}, fmt.Errorf("unknown view %q: use summary, timeline, recent, closest or all", q.View)
	}
	r := Report{Found: found, View: view, Section: section, Query: q}
	if !found {
		return r, nil
	}
	r.Catalog = newCatalog(section.Categories)
	for _, cat := range section.Categories {
		if r.Catalog.matches(cat.ID, q.Category) {
			r.Categories = append(r.Categories, cat)
		}
	}
	for _, a := range section.List {
		if nameMatches(a.Name, q.Query) && r.Catalog.matches(a.CategoryID, q.Category) {
			r.Matched = append(r.Matched, a)
			if a.Completed {
				r.MatchedCompleted++
			}
		}
	}
	r.applyView(limitFor(view, q.Limit))
	return r, nil
}

func limitFor(view string, limit int) int {
	if view == ViewSummary {
		return summaryRecent
	}
	if limit > 0 {
		return limit
	}
	return defaultLimits[view]
}

func (r *Report) applyView(limit int) {
	var list []Achievement
	switch r.View {
	case ViewSummary, ViewRecent:
		list = r.dated(true)
	case ViewTimeline:
		list = r.dated(false)
		if len(list) > 0 {
			r.Earliest, r.Latest = &list[0], &list[len(list)-1]
		}
	case ViewClosest:
		list = r.closest()
	case ViewAll:
		list = r.Matched
	}
	r.ListedTotal = len(list)
	if len(list) > limit {
		if r.View == ViewTimeline {
			list = list[len(list)-limit:] // the latest ones, still in date order
		} else {
			list = list[:limit]
		}
	}
	r.Listed = list
}

// dated returns the completed achievements with a date, oldest first or newest first.
func (r *Report) dated(newestFirst bool) []Achievement {
	var list []Achievement
	for _, a := range r.Matched {
		switch {
		case !a.Completed:
		case a.CompletedOn == "":
			r.Undated++
		default:
			list = append(list, a)
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].CompletedOn != list[j].CompletedOn {
			return (list[i].CompletedOn < list[j].CompletedOn) != newestFirst
		}
		return list[i].ID < list[j].ID
	})
	return list
}

// closest ranks the incomplete achievements by progress, highest first; those without criteria data are counted.
func (r *Report) closest() []Achievement {
	var list []Achievement
	for _, a := range r.Matched {
		if a.Completed {
			continue
		}
		if !a.Progress().Known {
			r.WithoutProgress++
			continue
		}
		list = append(list, a)
	}
	sort.SliceStable(list, func(i, j int) bool {
		pi, pj := list[i].Progress().Fraction, list[j].Progress().Fraction
		if pi != pj {
			return pi > pj
		}
		return list[i].Name < list[j].Name
	})
	return list
}
