// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package mail covers the mailbox from the last time it was opened: letters, gold, COD, attachments, expiry.
package mail

import (
	"fmt"
	"time"

	"wowmcp/internal/snapshot"
)

// expiringSoon: a letter expiring sooner than this goes on the expiringSoon list.
const expiringSoon = 72 * time.Hour

// View describes the mailbox with the time to expiry of each letter.
func View(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	mail := s.Map("mail")
	if mail == nil {
		return map[string]any{"note": "No saved mail. Open the mailbox in game, then type /mcp reload."}
	}
	var money, cod int64
	messages := []any{}
	var expiring []string
	for _, raw := range snapshot.AsList(mail["messages"]) {
		m := snapshot.AsMap(raw)
		e := map[string]any{"sender": m["sender"], "subject": m["subject"]}
		if v, ok := snapshot.AsInt(m["moneyCopper"]); ok && v > 0 {
			e["money"] = snapshot.Money(v).String()
			money += v
		}
		if v, ok := snapshot.AsInt(m["codCopper"]); ok && v > 0 {
			e["cod"] = snapshot.Money(v).String()
			cod += v
		}
		if items := snapshot.AsList(m["items"]); len(items) > 0 {
			e["items"] = attachmentNames(items)
		}
		if m["returned"] == true {
			e["returned"] = true
		}
		if m["read"] != true {
			e["unread"] = true
		}
		if at, ok := snapshot.AsInt(m["expiresAt"]); ok {
			left, gone := fresh.Until(at)
			if gone {
				e["expired"] = true
				e["note"] = "expired since the dump: the letter went back to the sender or disappeared"
			} else {
				e["expiresIn"] = snapshot.FormatAge(left)
				if left < expiringSoon {
					expiring = append(expiring, fmt.Sprintf("%v (%s)", m["subject"], snapshot.FormatAge(left)))
				}
			}
		}
		messages = append(messages, e)
	}
	out := map[string]any{"messages": messages, "count": len(messages)}
	if money > 0 {
		out["totalMoney"] = snapshot.Money(money).String()
	}
	if cod > 0 {
		out["totalCOD"] = snapshot.Money(cod).String()
	}
	if len(expiring) > 0 {
		out["expiringSoon"] = expiring
	}
	if total, ok := snapshot.AsInt(mail["totalOnServer"]); ok && total > int64(len(messages)) {
		out["moreOnServer"] = total - int64(len(messages))
	}
	snapshot.Merge(out, fresh.CapturedMeta(mail))
	return out
}

// attachmentNames lists the letter's attachments: "Linen Cloth x20".
func attachmentNames(items []any) []string {
	var names []string
	for _, ri := range items {
		it := snapshot.AsMap(ri)
		n, ok := snapshot.AsInt(it["count"])
		if ok && n > 1 {
			names = append(names, fmt.Sprintf("%v x%d", it["name"], n))
		} else {
			names = append(names, fmt.Sprint(it["name"]))
		}
	}
	return names
}
