// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package shopping

import (
	"fmt"
	"math"

	"wowmcp/internal/game/ammo"
	"wowmcp/internal/game/character"
	"wowmcp/internal/game/consumption"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/game/trainer"
	"wowmcp/internal/snapshot"
)

// lowFreeSlots: this many free slots or fewer means almost full bags.
const lowFreeSlots = 3

// noData is the text for missing data in forecasts.
const noData = "no data"

// ReturnToTown answers the question "when to return to town": in how many levels there are new trainer skills,
// in how many hours of play the bags fill up, for how many hours the ammo lasts and what the gear durability is.
// When there is too little history for a rate, the relevant field says "no data".
func ReturnToTown(name string, s snapshot.Snapshot, tr progress.Tracker, fresh snapshot.Freshness) map[string]any {
	out := map[string]any{}
	var now []string
	soonest, soonestWhy := math.Inf(1), ""

	level, _ := s.LevelAndMoney()
	trainerView, trainerNow := nextTrainerSkills(s, level)
	out["trainer"] = trainerView
	if trainerNow != "" {
		now = append(now, trainerNow)
	}

	bags, bagHours, bagNow := bagsForecast(name, s, tr)
	out["bags"] = bags
	if bagNow != "" {
		now = append(now, bagNow)
	}
	if bagHours >= 0 && bagHours < soonest {
		soonest, soonestWhy = bagHours, "bags"
	}

	if f := ammo.Left(name, s, tr); f.Present {
		a := map[string]any{"name": f.Stock.Name, "total": f.Stock.Total()}
		switch {
		case !f.RateKnown:
			a["hoursLeft"] = noData
			a["note"] = f.Note
		case !f.Burning:
			a["hoursLeft"] = noData
			a["note"] = "ammo did not decrease in the measured stretch of history"
		default:
			a["hoursLeft"] = snapshot.Round1(f.HoursLeft)
			a["timeLeft"] = snapshot.FormatSeconds(int64(f.HoursLeft * 3600))
			if f.HoursLeft < 1 {
				now = append(now, "ammo lasts less than an hour")
			}
			if f.HoursLeft < soonest {
				soonest, soonestWhy = f.HoursLeft, "ammo"
			}
		}
		out["ammo"] = a
	}

	if d := character.Durability(s, fresh); d != nil {
		dv := map[string]any{"overallPercent": d["overallPercent"], "lowest": d["lowest"]}
		if broken, ok := d["nearlyBroken"]; ok {
			dv["nearlyBroken"] = broken
			now = append(now, "gear close to breaking (below 25%)")
		}
		out["durability"] = dv
	}

	if len(now) > 0 {
		out["returnNow"] = now
	}
	if !math.IsInf(soonest, 1) {
		out["soonestHours"] = snapshot.Round1(soonest)
		out["soonestBecause"] = soonestWhy
	} else {
		out["soonestHours"] = noData
	}
	out["note"] = "Forecasts rely on the rate from dump history (the same window as wow_progress), so they are approximations; " +
		"purchases and sales between dumps distort the bag-filling rate."
	return out
}

// nextTrainerSkills points to the nearest level at which something new unlocks at the remembered trainers,
// and says whether anything can be bought right now.
func nextTrainerSkills(s snapshot.Snapshot, level int64) (view map[string]any, now string) {
	offers := trainer.GoldOffers(s)
	if len(offers) == 0 {
		return map[string]any{"note": noData + ": no saved trainer offers (open a trainer window in game and run /mcp reload)"}, ""
	}
	var availableNow []string
	nextLevel := int64(math.MaxInt64)
	type upcoming struct{ label, npc string }
	byLevel := map[int64][]upcoming{}
	for _, v := range offers {
		for _, raw := range snapshot.AsList(v.Section["services"]) {
			svc := snapshot.AsMap(raw)
			cat, _ := svc["category"].(string)
			if cat == "used" {
				continue
			}
			label, _ := svc["name"].(string)
			if rank, _ := svc["rank"].(string); rank != "" {
				label += " " + rank
			}
			req, _ := snapshot.AsInt(svc["levelRequired"])
			switch {
			case cat == "available":
				availableNow = appendUnique(availableNow, label)
			case req > level:
				byLevel[req] = append(byLevel[req], upcoming{label, v.Npc})
				nextLevel = min(nextLevel, req)
			}
		}
	}
	view = map[string]any{}
	if len(availableNow) > 0 {
		view["availableNow"] = availableNow
		now = fmt.Sprintf("skills to buy from the trainer (%d)", len(availableNow))
	}
	if nextLevel == math.MaxInt64 {
		view["next"] = noData + ": the remembered offers have no services with a higher required level"
		return view, now
	}
	var labels, npcs []string
	for _, u := range byLevel[nextLevel] {
		labels = appendUnique(labels, u.label)
		npcs = appendUnique(npcs, u.npc)
	}
	view["next"] = map[string]any{"level": nextLevel, "levelsAway": nextLevel - level, "skills": labels, "trainers": npcs}
	return view, now
}

func appendUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

// bagsForecast says how many slots are free and in how many hours of play the bags fill up (from the growth of used slots in history).
// The second result is the hours to fill, or -1 when unknown.
func bagsForecast(name string, s snapshot.Snapshot, tr progress.Tracker) (map[string]any, float64, string) {
	var total, free int64
	for _, raw := range s.Bags() {
		b := snapshot.AsMap(raw)
		n, _ := snapshot.AsInt(b["size"])
		f, _ := snapshot.AsInt(b["free"])
		total, free = total+n, free+f
	}
	out := map[string]any{"freeSlots": free, "totalSlots": total}
	var now string
	if free <= lowFreeSlots {
		now = fmt.Sprintf("bags almost full (free slots: %d)", free)
	}
	hours := -1.0
	w, err := consumption.Load(tr, name, s)
	if err == nil {
		var use consumption.Usage
		use, err = w.MeasureGain(usedSlots)
		if err == nil {
			out["gainedSlotsPerHour"] = snapshot.Round1(use.PerHour)
			out["window"] = use.Window
			if left, ok := use.HoursLeft(free); ok {
				hours = left
				out["hoursUntilFull"] = snapshot.Round1(left)
				out["timeUntilFull"] = snapshot.FormatSeconds(int64(left * 3600))
			} else {
				out["hoursUntilFull"] = noData
				out["note"] = "used bag slots did not grow in the measured stretch of history"
			}
		}
	}
	if err != nil {
		out["hoursUntilFull"] = noData
		out["note"] = err.Error()
	}
	return out, hours, now
}

// usedSlots counts the used bag slots of the dump (size minus free).
func usedSlots(s snapshot.Snapshot) int64 {
	var used int64
	for _, raw := range s.Bags() {
		b := snapshot.AsMap(raw)
		n, _ := snapshot.AsInt(b["size"])
		f, _ := snapshot.AsInt(b["free"])
		used += n - f
	}
	return used
}
