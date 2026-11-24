// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package achievements

import "wowmcp/internal/snapshot"

func asInt(v any) int64 {
	n, _ := snapshot.AsInt(v)
	return n
}

func asText(v any) string {
	s, _ := v.(string)
	return s
}

func readAchievements(s snapshot.Snapshot) (Section, bool) {
	m := s.Map("achievements")
	if m == nil {
		return Section{}, false
	}
	section := Section{NotListed: asInt(m["notListed"]), CriteriaSkipped: asInt(m["criteriaSkipped"])}
	section.Points, section.HasPoints = snapshot.AsInt(m["points"])
	var hasTotal, hasCompleted bool
	section.Total, hasTotal = snapshot.AsInt(m["total"])
	section.Completed, hasCompleted = snapshot.AsInt(m["completed"])
	section.HasCounts = hasTotal && hasCompleted
	for _, raw := range snapshot.AsList(m["categories"]) {
		c := snapshot.AsMap(raw)
		section.Categories = append(section.Categories, Category{
			ID: asInt(c["id"]), ParentID: asInt(c["parentID"]), Name: asText(c["name"]),
			Total: asInt(c["total"]), Completed: asInt(c["completed"]),
		})
	}
	for _, raw := range snapshot.AsList(m["list"]) {
		section.List = append(section.List, readAchievement(snapshot.AsMap(raw)))
	}
	return section, true
}

func readAchievement(m map[string]any) Achievement {
	a := Achievement{
		ID: asInt(m["id"]), Points: asInt(m["points"]), CategoryID: asInt(m["category"]),
		Name: asText(m["name"]), Description: asText(m["description"]), Reward: asText(m["reward"]),
		CompletedOn:   asText(m["completedOn"]),
		CriteriaTotal: asInt(m["criteriaTotal"]), CriteriaRead: asInt(m["criteriaRead"]), CriteriaDone: asInt(m["criteriaDone"]),
	}
	a.Completed, _ = m["completed"].(bool)
	for _, raw := range snapshot.AsList(m["criteria"]) {
		c := snapshot.AsMap(raw)
		criterion := Criterion{Text: asText(c["text"]), Required: asInt(c["required"])}
		criterion.Completed, _ = c["completed"].(bool)
		criterion.Quantity, criterion.HasQuantity = snapshot.AsInt(c["quantity"])
		a.Criteria = append(a.Criteria, criterion)
	}
	return a
}

func readStatistics(s snapshot.Snapshot) (Statistics, bool) {
	m := s.Map("statistics")
	if m == nil {
		return Statistics{}, false
	}
	section := Statistics{Empty: asInt(m["empty"]), NotListed: asInt(m["notListed"])}
	for _, raw := range snapshot.AsList(m["categories"]) {
		c := snapshot.AsMap(raw)
		category := StatisticsCategory{ID: asInt(c["id"]), ParentID: asInt(c["parentID"]), Name: asText(c["name"])}
		for _, rawStat := range snapshot.AsList(c["list"]) {
			st := snapshot.AsMap(rawStat)
			category.List = append(category.List, Statistic{ID: asInt(st["id"]), Name: asText(st["name"]), Value: asText(st["value"])})
		}
		section.Categories = append(section.Categories, category)
	}
	return section, true
}
