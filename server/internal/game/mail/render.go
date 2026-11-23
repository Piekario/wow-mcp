// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package mail

import (
	"fmt"

	"wowmcp/internal/snapshot"
)

// View is the answer of wow_mail.
func View(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	return Inspect(s, fresh).Render(fresh)
}

func (r Report) Render(fresh snapshot.Freshness) map[string]any {
	if !r.Saved {
		return map[string]any{"note": "No saved mail. Open the mailbox in game, then type /mcp reload."}
	}
	messages := []any{}
	for _, letter := range r.Letters {
		messages = append(messages, letter.render())
	}
	out := map[string]any{"messages": messages, "count": len(messages)}
	if r.Money > 0 {
		out["totalMoney"] = snapshot.Money(r.Money).String()
	}
	if r.COD > 0 {
		out["totalCOD"] = snapshot.Money(r.COD).String()
	}
	if soon := r.ExpiringSoonLabels(); len(soon) > 0 {
		out["expiringSoon"] = soon
	}
	if r.MoreOnServer > 0 {
		out["moreOnServer"] = r.MoreOnServer
	}
	snapshot.Merge(out, fresh.CapturedMetaOf(r.Captured))
	return out
}

// ExpiringSoonLabels names the letters about to expire: "subject (time left)".
func (r Report) ExpiringSoonLabels() []string {
	var labels []string
	for _, letter := range r.ExpiringSoon() {
		labels = append(labels, fmt.Sprintf("%v (%s)", letter.Subject, snapshot.FormatAge(letter.Expiry.Left)))
	}
	return labels
}

func (l LetterStatus) render() map[string]any {
	e := map[string]any{"sender": l.Sender, "subject": l.Subject}
	if l.Money > 0 {
		e["money"] = snapshot.Money(l.Money).String()
	}
	if l.COD > 0 {
		e["cod"] = snapshot.Money(l.COD).String()
	}
	if len(l.Attachments) > 0 {
		e["items"] = attachmentLabels(l.Attachments)
	}
	if l.Returned {
		e["returned"] = true
	}
	if !l.Read {
		e["unread"] = true
	}
	switch {
	case !l.Expiry.Known:
	case l.Expiry.Expired:
		e["expired"] = true
		e["note"] = "expired since the dump: the letter went back to the sender or disappeared"
	default:
		e["expiresIn"] = snapshot.FormatAge(l.Expiry.Left)
	}
	return e
}

// "Linen Cloth x20"; a single item has no count.
func attachmentLabels(attachments []Attachment) []string {
	var labels []string
	for _, a := range attachments {
		if a.Count > 1 {
			labels = append(labels, fmt.Sprintf("%v x%d", a.Name, a.Count))
		} else {
			labels = append(labels, fmt.Sprint(a.Name))
		}
	}
	return labels
}
