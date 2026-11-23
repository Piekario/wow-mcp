// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package mail

import "wowmcp/internal/snapshot"

func read(s snapshot.Snapshot) (Mailbox, bool) {
	section := s.Map("mail")
	if section == nil {
		return Mailbox{}, false
	}
	box := Mailbox{Captured: snapshot.CapturedOf(section)}
	box.TotalOnServer, _ = snapshot.AsInt(section["totalOnServer"])
	for _, raw := range snapshot.AsList(section["messages"]) {
		box.Letters = append(box.Letters, readLetter(snapshot.AsMap(raw)))
	}
	return box, true
}

func readLetter(m map[string]any) Letter {
	letter := Letter{Sender: m["sender"], Subject: m["subject"], Returned: m["returned"] == true, Read: m["read"] == true}
	if v, ok := snapshot.AsInt(m["moneyCopper"]); ok && v > 0 {
		letter.Money = v
	}
	if v, ok := snapshot.AsInt(m["codCopper"]); ok && v > 0 {
		letter.COD = v
	}
	letter.ExpiresAt, letter.HasExpiry = snapshot.AsInt(m["expiresAt"])
	for _, raw := range snapshot.AsList(m["items"]) {
		item := snapshot.AsMap(raw)
		count, _ := snapshot.AsInt(item["count"])
		letter.Attachments = append(letter.Attachments, Attachment{Name: item["name"], Count: count})
	}
	return letter
}
