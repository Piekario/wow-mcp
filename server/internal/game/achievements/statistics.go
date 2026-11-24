// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package achievements

// StatisticsQuery is the input of wow_statistics.
type StatisticsQuery struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Query     string `json:"query,omitempty" jsonschema:"optional fragment of the statistic name (case-insensitive), e.g. deaths, gold, quests"`
	Category  string `json:"category,omitempty" jsonschema:"optional fragment of the category name (case-insensitive); a parent category also matches its subcategories"`
}

// Statistic is one counter with its value as the game shows it ("62", "5g 20s 3c").
type Statistic struct {
	ID          int64
	Name, Value string
}

// StatisticsCategory holds the statistics of one category that have a value.
type StatisticsCategory struct {
	ID, ParentID int64
	Name         string
	List         []Statistic
}

// Statistics is the dump's statistics section. Empty counts the statistics the game shows as "--".
type Statistics struct {
	Categories       []StatisticsCategory
	Empty, NotListed int64
}

// StatisticsReport is the result of wow_statistics.
type StatisticsReport struct {
	Found      bool
	Categories []StatisticsCategory // filtered; each with at least one statistic
	Shown      int
	Section    Statistics
}

// AnalyseStatistics filters the statistics by name and category.
func AnalyseStatistics(section Statistics, found bool, q StatisticsQuery) StatisticsReport {
	r := StatisticsReport{Found: found, Section: section}
	if !found {
		return r
	}
	var all []Category
	for _, c := range section.Categories {
		all = append(all, Category{ID: c.ID, ParentID: c.ParentID, Name: c.Name})
	}
	catalog := newCatalog(all)
	for _, c := range section.Categories {
		if !catalog.matches(c.ID, q.Category) {
			continue
		}
		kept := StatisticsCategory{ID: c.ID, ParentID: c.ParentID, Name: catalog.Path(c.ID)}
		for _, st := range c.List {
			if nameMatches(st.Name, q.Query) {
				kept.List = append(kept.List, st)
			}
		}
		if len(kept.List) > 0 {
			r.Categories = append(r.Categories, kept)
			r.Shown += len(kept.List)
		}
	}
	return r
}
