// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package shopping

import (
	"math"

	"wowmcp/internal/game/ammo"
	"wowmcp/internal/game/character"
	"wowmcp/internal/game/consumption"
	"wowmcp/internal/game/items"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// lowFreeSlots: this many free slots or fewer means almost full bags.
const lowFreeSlots = 3

// What limits the time before the player has to return to town.
const (
	LimitedByBags = "bags"
	LimitedByAmmo = "ammo"
)

// NextSkills are the skills that unlock at the nearest higher level at the remembered trainers.
type NextSkills struct {
	Level, LevelsAway int64
	Skills, Trainers  []string
}

// TrainerOutlook says in how many levels there are new trainer skills and whether anything can be bought right now.
type TrainerOutlook struct {
	NoOffers     bool
	AvailableNow []string
	Next         *NextSkills // nil: the remembered offers have nothing for a higher level
}

// BagForecast says how many slots are free and in how many hours of play the bags fill up (from the growth of used slots in history).
type BagForecast struct {
	Slots             items.BagSlots
	Use               consumption.Usage
	Measured          bool
	HoursUntilFull    float64
	HasHoursUntilFull bool
	Err               string // why there is no rate
}

// Departure answers "when to return to town": new trainer skills, bags filling up, ammo running out and gear
// durability. When there is too little history for a rate, the relevant field has no data.
type Departure struct {
	Trainer    TrainerOutlook
	Bags       BagForecast
	Ammo       ammo.Forecast
	Durability *character.DurabilityReport
	// Soonest is the hours until the nearest limit that has a rate, and what that limit is.
	Soonest        float64
	HasSoonest     bool
	SoonestBecause string
}

func (d Departure) BagsAlmostFull() bool { return d.Bags.Slots.Free <= lowFreeSlots }

// AmmoUnderHour: ammo is being used up and lasts less than an hour of play.
func (d Departure) AmmoUnderHour() bool {
	return d.Ammo.Present && d.Ammo.Burning && d.Ammo.HoursLeft < 1
}

func (d Departure) GearNearlyBroken() bool {
	return d.Durability != nil && len(d.Durability.NearlyBroken) > 0
}

func planDeparture(name string, s snapshot.Snapshot, tr progress.Tracker) Departure {
	level, _ := s.LevelAndMoney()
	d := Departure{Trainer: outlookTrainer(readOffers(s), level), Bags: forecastBags(name, s, tr), Ammo: ammo.Left(name, s, tr)}
	if report, ok := character.InspectDurability(s); ok {
		d.Durability = &report
	}
	soonest := math.Inf(1)
	if d.Bags.HasHoursUntilFull && d.Bags.HoursUntilFull < soonest {
		soonest, d.SoonestBecause = d.Bags.HoursUntilFull, LimitedByBags
	}
	if d.Ammo.Present && d.Ammo.RateKnown && d.Ammo.Burning && d.Ammo.HoursLeft < soonest {
		soonest, d.SoonestBecause = d.Ammo.HoursLeft, LimitedByAmmo
	}
	d.Soonest, d.HasSoonest = soonest, !math.IsInf(soonest, 1)
	return d
}

// outlookTrainer points to the nearest level at which something new unlocks at the remembered trainers,
// and says whether anything can be bought right now.
func outlookTrainer(offers []TrainerOffer, level int64) TrainerOutlook {
	if len(offers) == 0 {
		return TrainerOutlook{NoOffers: true}
	}
	var out TrainerOutlook
	nextLevel := int64(math.MaxInt64)
	type upcoming struct{ label, npc string }
	byLevel := map[int64][]upcoming{}
	for _, o := range offers {
		for _, sv := range o.Services {
			switch {
			case sv.Category == "used":
			case sv.Category == "available":
				out.AvailableNow = appendUnique(out.AvailableNow, sv.Label())
			case sv.LevelRequired > level:
				byLevel[sv.LevelRequired] = append(byLevel[sv.LevelRequired], upcoming{sv.Label(), o.Npc})
				nextLevel = min(nextLevel, sv.LevelRequired)
			}
		}
	}
	if nextLevel == math.MaxInt64 {
		return out
	}
	next := &NextSkills{Level: nextLevel, LevelsAway: nextLevel - level}
	for _, u := range byLevel[nextLevel] {
		next.Skills = appendUnique(next.Skills, u.label)
		next.Trainers = appendUnique(next.Trainers, u.npc)
	}
	out.Next = next
	return out
}

func appendUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

func forecastBags(name string, s snapshot.Snapshot, tr progress.Tracker) BagForecast {
	f := BagForecast{Slots: items.Space(s)}
	w, err := consumption.Load(tr, name, s)
	if err == nil {
		var use consumption.Usage
		if use, err = w.MeasureGain(func(snap snapshot.Snapshot) int64 {
			slots := items.Space(snap)
			return slots.Total - slots.Free
		}); err == nil {
			f.Use, f.Measured = use, true
			f.HoursUntilFull, f.HasHoursUntilFull = use.HoursLeft(f.Slots.Free)
		}
	}
	if err != nil {
		f.Err = err.Error()
	}
	return f
}
