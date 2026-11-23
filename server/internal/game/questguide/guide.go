// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package questguide

import "wowmcp/internal/snapshot"

// Tag is a quest tag of a guide step: what to do with which quest.
type Tag struct {
	ID        int64
	Action    string
	Objective any
}

// Step is a line of the player's Guidelime guide.
type Step struct {
	Line             any
	Text             string
	Optional, Passed bool
	Tags             []Tag
}

// GuideData is the guide section.
type GuideData struct {
	Name                string
	Addon, NextGuide    any
	TextUnavailable     bool
	Total, Done, Passed int64
	Next                []Step
}

// TagAnswer is the quest part of a step. Entry is nil when the quest database does not know the quest.
type TagAnswer struct {
	Tag    Tag
	Status string
	Entry  *Entry
	Where  []Place
}

// StepAnswer is a guide step with its quests placed on the map.
type StepAnswer struct {
	Step   Step
	Quests []TagAnswer
}

// Availability says why a guide answer is empty.
type Availability int

const (
	GuideAvailable Availability = iota
	NoGuide
	NoGuideText
)

// GuideResult is the next steps of the player's leveling guide with quest names and places.
type GuideResult struct {
	Availability Availability
	Guide        GuideData
	Player       Player
	Steps        []StepAnswer
	Names        map[int64]string
	Finished     bool
}

// Next reads the guide and places the quests of its next steps.
func Next(s snapshot.Snapshot) GuideResult {
	g := readGuide(s)
	r := GuideResult{Guide: g}
	switch {
	case g.Name == "":
		r.Availability = NoGuide
		return r
	case g.TextUnavailable:
		r.Availability = NoGuideText
		return r
	}
	r.Player = readPlayer(s)
	states := logStates(s)
	index := questIndex(readDatabase(s))
	r.Names = map[int64]string{}
	for id, e := range index {
		r.Names[id], _ = e.Name.(string)
	}
	for _, step := range g.Next {
		answer := StepAnswer{Step: step}
		for _, tag := range step.Tags {
			answer.Quests = append(answer.Quests, answerTag(tag, index, states, r.Player))
		}
		r.Steps = append(r.Steps, answer)
	}
	r.Finished = r.Steps == nil && g.Total > 0 && g.Done >= g.Total
	return r
}

// questIndex maps quest ID -> its questDb entry.
func questIndex(db Database) map[int64]Entry {
	index := map[int64]Entry{}
	for _, e := range db.Quests {
		if id, ok := snapshot.AsInt(e.ID); ok {
			index[id] = e
		}
	}
	return index
}

func answerTag(tag Tag, index map[int64]Entry, states map[int64]string, p Player) TagAnswer {
	a := TagAnswer{Tag: tag, Status: statusOf(states, tag.ID)}
	e, known := index[tag.ID]
	if !known {
		return a
	}
	a.Entry = &e
	switch tag.Action {
	case "accept":
		a.Where = places(e.StartedBy, p)
	case "turnin":
		a.Where = places(e.FinishedBy, p)
	case "complete":
		a.Where = places(e.Objectives, p)
	}
	return a
}
