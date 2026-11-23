// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package altmail suggests what to mail between an account's characters: materials
// lying in one character's bags and bank that are ingredients of another character's recipes.
// The server only reads data and sends nothing; the player sends the parcel in game.
package altmail

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"wowmcp/internal/game/items"
	"wowmcp/internal/game/professions"
	"wowmcp/internal/snapshot"
)

// Query is the input of wow_alt_mail.
type Query struct {
	From  string `json:"from,omitempty" jsonschema:"only materials of this character (sender); empty = all characters"`
	To    string `json:"to,omitempty" jsonschema:"only for this character (recipient); empty = all characters"`
	Query string `json:"query,omitempty" jsonschema:"material name fragment"`
}

// The kind of evidence that the recipient needs the material.
const (
	EvidenceRecipe     = "recipe"     // the material is an ingredient of a known recipe (the recipes section)
	EvidenceProfession = "profession" // only a match of the item class to the profession: weaker evidence
)

// professionKinds says which Trade Goods kind (itemkind.TradeGoodsKind) a profession consumes, by skill line ID.
// Only the obvious links from classic WoW (leather - Leatherworking, cloth - Tailoring, metal and stone -
// Blacksmithing, Engineering, herbs - Alchemy, enchanting ingredients, meat and fish - Cooking). We skip everything
// else so as not to invent mechanics.
var professionKinds = map[int64][]string{
	professions.SkillLeatherworking: {"Leather"},
	professions.SkillTailoring:      {"Cloth"},
	professions.SkillBlacksmithing:  {"Metal & Stone"},
	professions.SkillEngineering:    {"Metal & Stone"},
	professions.SkillAlchemy:        {"Herb"},
	professions.SkillEnchanting:     {"Enchanting"},
	professions.SkillCooking:        {"Cooking"},
}

// weakProfession is a recipient's profession without saved recipes: its name in the dump and its skill line ID.
type weakProfession struct {
	name  string
	skill int64
}

// recipient is what the plan needs to know about the character that would receive the parcel.
type recipient struct {
	name string
	uses map[string][]string // material (lowercase) -> the recipes that use it
	weak []weakProfession
	held map[string]*items.ItemStock
}

// Material is one stack worth sending and the reason the recipient needs it.
type Material struct {
	Name           string
	Count          int64
	Where          []string
	Evidence       string
	Reasons        []string
	RecipientHas   int64
	RecipientHolds bool
}

func (m Material) Weak() bool { return m.Evidence == EvidenceProfession }

type Transfer struct {
	From, To  string
	Materials []Material
}

type CharacterAge struct {
	Name    string
	Minutes any
}

// Proposal is what to mail between the characters. Comparable is false when the account has fewer than two characters.
type Proposal struct {
	Comparable     bool
	Transfers      []Transfer
	AnyRecipes     bool
	AnyWeak        bool
	MissingRecipes []string
	Ages           []CharacterAge
}

func whereList(st *items.ItemStock) []string {
	var out []string
	if st.Bags > 0 {
		out = append(out, "bags")
	}
	if st.Bank > 0 {
		out = append(out, "bank")
	}
	return out
}

// need describes why the recipient needs the material.
type need struct {
	evidence string
	reasons  []string
}

// needFor rates a material against the recipient; false when they do not need it.
func needFor(st *items.ItemStock, uses map[string][]string, weak []weakProfession) (need, bool) {
	if recipes := uses[strings.ToLower(st.Item.Name)]; len(recipes) > 0 {
		return need{EvidenceRecipe, recipes}, true
	}
	kind := st.Item.TradeGoodsKind()
	if kind == "" {
		return need{}, false
	}
	label := kind
	if sub := st.Item.SubType; sub != "" && sub != kind {
		label = sub // the client's own subtype name (e.g. Stoff on a German client)
	}
	var reasons []string
	for _, prof := range weak {
		if slices.Contains(professionKinds[prof.skill], kind) {
			reasons = append(reasons, fmt.Sprintf("%s (no saved recipes, matched by kind %q)", prof.name, label))
		}
	}
	if len(reasons) == 0 {
		return need{}, false
	}
	return need{EvidenceProfession, reasons}, true
}

func resolveFilter(db *snapshot.Database, name string) (string, error) {
	if name == "" {
		return "", nil
	}
	resolved, _, err := db.Lookup(name)
	return resolved, err
}

func analyse(db *snapshot.Database, q Query, fresh snapshot.Freshness) (Proposal, error) {
	if db == nil || len(db.Characters) < 2 {
		return Proposal{}, nil
	}
	from, err := resolveFilter(db, q.From)
	if err != nil {
		return Proposal{}, err
	}
	to, err := resolveFilter(db, q.To)
	if err != nil {
		return Proposal{}, err
	}
	query := strings.ToLower(q.Query)
	names := db.CharacterNames()

	p := Proposal{Comparable: true}
	for _, sender := range names {
		if from != "" && sender != from {
			continue
		}
		mats := sendable(db.Characters[sender])
		for _, name := range names {
			if name == sender || (to != "" && name != to) {
				continue
			}
			r := readRecipient(name, db.Characters[name], fresh)
			p.noteRecipient(r)
			if materials := materialsFor(mats, r, query); len(materials) > 0 {
				p.Transfers = append(p.Transfers, Transfer{From: sender, To: name, Materials: materials})
			}
		}
	}
	sort.SliceStable(p.Transfers, func(i, j int) bool {
		return len(p.Transfers[i].Materials) > len(p.Transfers[j].Materials)
	})
	for _, n := range names {
		if minutes := fresh.SnapshotMeta(n, db.Characters[n])["ageMinutes"]; minutes != nil {
			p.Ages = append(p.Ages, CharacterAge{n, minutes})
		}
	}
	return p, nil
}

func (p *Proposal) noteRecipient(r recipient) {
	if len(r.uses) > 0 {
		p.AnyRecipes = true
	}
	if len(r.weak) == 0 {
		return
	}
	p.AnyWeak = true
	names := make([]string, len(r.weak))
	for i, w := range r.weak {
		names[i] = w.name
	}
	if label := fmt.Sprintf("%s: %v", r.name, names); !slices.Contains(p.MissingRecipes, label) {
		p.MissingRecipes = append(p.MissingRecipes, label)
	}
}

// materialsFor lists, in name order, the sender's stacks the recipient needs.
func materialsFor(mats map[string]*items.ItemStock, r recipient, query string) []Material {
	var out []Material
	for _, key := range slices.Sorted(maps.Keys(mats)) {
		st := mats[key]
		name := st.Item.Name
		if query != "" && !strings.Contains(strings.ToLower(name), query) {
			continue
		}
		n, ok := needFor(st, r.uses, r.weak)
		if !ok {
			continue
		}
		m := Material{Name: name, Count: st.Total(), Where: whereList(st), Evidence: n.evidence, Reasons: n.reasons}
		if h := r.held[strings.ToLower(name)]; h != nil {
			m.RecipientHas, m.RecipientHolds = h.Total(), true
		}
		out = append(out, m)
	}
	return out
}
