// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package mail reads the mailbox saved when the player last opened it and says which letters expire soon.
package mail

import (
	"time"

	"wowmcp/internal/snapshot"
)

// ExpiringSoon: a letter with less time left than this needs the player's attention.
const ExpiringSoon = 72 * time.Hour

type Attachment struct {
	Name  any
	Count int64
}

// Letter keeps Sender and Subject as the client gave them (they may be missing), so the answer shows
// exactly what the dump holds.
type Letter struct {
	Sender, Subject any
	Money, COD      int64
	Attachments     []Attachment
	Returned, Read  bool
	ExpiresAt       int64
	HasExpiry       bool
}

// Mailbox is the mail section: the letters the client had loaded and how many the game server holds.
type Mailbox struct {
	Letters       []Letter
	TotalOnServer int64
	Captured      snapshot.Captured
}

// Expiry is what is known about a letter's time limit at the moment of the question.
type Expiry struct {
	Known   bool
	Expired bool
	Left    time.Duration
}

func (e Expiry) Soon() bool { return e.Known && !e.Expired && e.Left < ExpiringSoon }

type LetterStatus struct {
	Letter
	Expiry Expiry
}

// Report is the mailbox judged against the clock. Saved is false when the player never opened the mailbox.
type Report struct {
	Saved        bool
	Letters      []LetterStatus
	Money, COD   int64
	MoreOnServer int64
	Captured     snapshot.Captured
}

func Inspect(s snapshot.Snapshot, fresh snapshot.Freshness) Report {
	box, ok := read(s)
	if !ok {
		return Report{}
	}
	r := Report{Saved: true, Captured: box.Captured}
	for _, letter := range box.Letters {
		r.Money += letter.Money
		r.COD += letter.COD
		r.Letters = append(r.Letters, LetterStatus{letter, expiryOf(letter, fresh)})
	}
	if more := box.TotalOnServer - int64(len(box.Letters)); more > 0 {
		r.MoreOnServer = more
	}
	return r
}

func expiryOf(letter Letter, fresh snapshot.Freshness) Expiry {
	if !letter.HasExpiry {
		return Expiry{}
	}
	left, gone := fresh.Until(letter.ExpiresAt)
	return Expiry{Known: true, Expired: gone, Left: left}
}

func (r Report) ExpiringSoon() []LetterStatus {
	var soon []LetterStatus
	for _, letter := range r.Letters {
		if letter.Expiry.Soon() {
			soon = append(soon, letter)
		}
	}
	return soon
}
