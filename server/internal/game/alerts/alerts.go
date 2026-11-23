// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package alerts gathers in one place everything that needs the player's attention: expiring
// mail, auctions about to end, damaged gear, full bags and addon problems.
// Shown in wow_status; an empty list means nothing is on fire.
package alerts

import "wowmcp/internal/snapshot"

// Warning thresholds.
const (
	lowFreeSlots = 2   // this many free bag slots or fewer
	slowDumpMs   = 200 // dump collected for longer than this many milliseconds
)

// Kinds of alert, for filtering.
const (
	KindUnloadedDump = "unloadedDump"
	KindMail         = "mail"
	KindAuctions     = "auctions"
	KindDurability   = "durability"
	KindBags         = "bags"
	KindSlowDump     = "slowDump"
	KindAddonErrors  = "addonErrors"
)

// Alert is a single warning: its kind (for filtering) and the text for the player.
type Alert struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

// Assessment is everything the alert rules look at, read from one dump.
type Assessment struct {
	Unloaded        bool
	MailExpiring    []string
	AuctionsEnding  []string
	LowDurability   []string
	BagSlots        int64
	BagFree         int64
	DumpMs          float64
	HasDumpMs       bool
	SlowestSections []string
	ErrorCount      int
}

// Assess reads the facts the alerts are built from.
func Assess(s snapshot.Snapshot, fresh snapshot.Freshness) Assessment { return read(s, fresh) }

func (a Assessment) BagsAlmostFull() bool { return a.BagSlots > 0 && a.BagFree <= lowFreeSlots }

func (a Assessment) SlowDump() bool { return a.HasDumpMs && a.DumpMs > slowDumpMs }
