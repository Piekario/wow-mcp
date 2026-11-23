// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package pet

import (
	"fmt"

	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/game/trainer"
	"wowmcp/internal/snapshot"
)

// beastTraining is the name of the profession window with the abilities a Hunter can teach the pet.
const beastTraining = "Beast Training"

func readPet(s snapshot.Snapshot) Pet {
	m := s.Map("pet")
	p := Pet{Present: m != nil, Summoned: m["exists"] == true}
	p.Shown = Display{
		Name: m["name"], Family: m["family"], Level: m["level"], Dead: m["dead"], DamagePercent: m["damagePercent"],
		LoyaltyRate: m["loyaltyRate"], Loyalty: m["loyalty"], FoodTypes: m["foodTypes"], HappinessInfo: m["happinessInfo"],
		LoyaltyInfo: m["loyaltyInfo"], TrainingPointsInfo: m["trainingPointsInfo"],
	}
	p.Level, _ = snapshot.AsInt(m["level"])
	p.Family, _ = m["family"].(string)
	p.Happiness, p.HasHappiness = snapshot.AsInt(m["happiness"])
	xp, ok1 := snapshot.AsInt(m["xp"])
	xpMax, ok2 := snapshot.AsInt(m["xpMax"])
	p.XP, p.XPMax, p.HasXP = xp, xpMax, ok1 && ok2
	p.Points.Total, p.Points.HasTotal = snapshot.AsInt(m["trainingPointsTotal"])
	p.Points.Spent, p.Points.HasSpent = snapshot.AsInt(m["trainingPointsSpent"])
	p.Points.Free, p.Points.HasFree = snapshot.AsInt(m["trainingPointsFree"])
	for _, raw := range snapshot.AsList(m["abilities"]) {
		p.Abilities = append(p.Abilities, readAbility(snapshot.AsMap(raw)))
	}
	for _, raw := range snapshot.AsList(m["actionBar"]) {
		a := snapshot.AsMap(raw)
		slot := BarSlot{Name: fmt.Sprint(a["name"])}
		slot.AutoCast, slot.HasAutoCast = a["autoCast"].(bool)
		p.Bar = append(p.Bar, slot)
	}
	return p
}

// readAbility: stances and commands are marked petAction by addon 0.9.0 (the spellbook entry type, the same in every
// language); older dumps only have the English subtext "Pet Stance" / "Pet Command" in rank.
func readAbility(m map[string]any) Ability {
	a := Ability{Name: fmt.Sprint(m["name"])}
	a.Rank, _ = m["rank"].(string)
	a.Control = m["petAction"] == true || a.Rank == "Pet Stance" || a.Rank == "Pet Command"
	return a
}

func readBagFood(s snapshot.Snapshot) []FoodStack {
	var out []FoodStack
	for _, bag := range s.Bags() {
		for _, raw := range snapshot.AsList(snapshot.AsMap(bag)["items"]) {
			item := snapshot.AsMap(raw)
			if itemkind.Read(item).IsFood() {
				out = append(out, FoodStack{Name: item["name"], Count: snapshot.ItemCount(item), ID: item["id"]})
			}
		}
	}
	return out
}

// readLessons reads the Beast Training window. The classic client shows it as a profession window (recipes), and
// Forever 1.60.1 as a trainer window without an NPC.
func readLessons(s snapshot.Snapshot) Lessons {
	raw := s.Map("recipes")[beastTraining]
	window := snapshot.AsMap(raw)
	if window == nil {
		list, found := readTrainerLessons(s)
		return Lessons{List: list, Found: found, FromTrainer: raw == nil}
	}
	lessons := Lessons{Found: true}
	for _, entry := range snapshot.AsList(window["recipes"]) {
		r := snapshot.AsMap(entry)
		name, _ := r["name"].(string)
		if name == "" {
			continue
		}
		rankText, _ := r["rank"].(string)
		diff, _ := r["difficulty"].(string)
		l := Lesson{Name: name, RankText: rankText, Rank: RankNumber(rankText), Used: diff == "used"}
		l.LevelRequired, _ = snapshot.AsInt(r["levelRequired"])
		l.Points, l.HasPoints = snapshot.AsInt(r["trainingPoints"])
		lessons.List = append(lessons.List, l)
	}
	return lessons
}

// readTrainerLessons reads the latest trainer window without an NPC (Beast Training in Forever). Assumption, not
// confirmed in game: the costCopper of such an offer is the pet's training points.
func readTrainerLessons(s snapshot.Snapshot) ([]Lesson, bool) {
	for _, v := range trainer.Offers(s) {
		if !trainer.IsPetTraining(v.Section) {
			continue
		}
		var out []Lesson
		for _, raw := range snapshot.AsList(v.Section["services"]) {
			svc := snapshot.AsMap(raw)
			name, _ := svc["name"].(string)
			if name == "" {
				continue
			}
			rankText, _ := svc["rank"].(string)
			category, _ := svc["category"].(string)
			l := Lesson{Name: name, RankText: rankText, Rank: RankNumber(rankText), Used: category == "used"}
			l.LevelRequired, _ = snapshot.AsInt(svc["levelRequired"])
			l.Points, l.HasPoints = snapshot.AsInt(svc["costCopper"])
			out = append(out, l)
		}
		return out, true
	}
	return nil, false
}

// readTrainerServices lists what the remembered trainers can teach or will teach later (available, then unavailable).
func readTrainerServices(s snapshot.Snapshot) []TrainerService {
	var out []TrainerService
	for _, v := range trainer.Offers(s) {
		for _, category := range []string{"available", "unavailable"} {
			for _, raw := range trainer.ServicesOf(v.Section, category) {
				svc := snapshot.AsMap(raw)
				t := TrainerService{Trainer: v.Npc, Category: category}
				t.Name, _ = svc["name"].(string)
				t.Rank, _ = svc["rank"].(string)
				t.Cost, t.HasCost = snapshot.AsInt(svc["costCopper"])
				t.LevelRequired, t.HasLevel = snapshot.AsInt(svc["levelRequired"])
				out = append(out, t)
			}
		}
	}
	return out
}

func readStable(s snapshot.Snapshot) (Stable, bool) {
	m := s.Map("stable")
	if m == nil {
		return Stable{}, false
	}
	st := Stable{Captured: snapshot.CapturedOf(m)}
	st.Slots, st.HasSlots = snapshot.AsInt(m["slots"])
	for _, raw := range snapshot.AsList(m["pets"]) {
		p := snapshot.AsMap(raw)
		sp := StabledPet{
			Shown: StabledDisplay{Slot: p["slot"], Name: p["name"], Level: p["level"], Family: p["family"],
				Specialization: p["specialization"], Loyalty: p["loyalty"], Active: p["active"]},
			WithCharacter: p["current"] == true,
			Active:        p["active"] == true,
		}
		sp.Level, sp.HasLevel = snapshot.AsInt(p["level"])
		sp.Family, _ = p["family"].(string)
		st.Pets = append(st.Pets, sp)
	}
	return st, true
}
