// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package doctor

import (
	"fmt"
	"strings"

	"wowmcp/internal/game/addon"
	"wowmcp/internal/snapshot"
)

func dumpFile(f Facts, fresh snapshot.Freshness) Check {
	c := Check{Check: "dump file", Status: OK}
	c.Detail = "Reading " + f.File
	if !f.FileModified.IsZero() {
		age := fresh.Now().Sub(f.FileModified)
		c.Detail += fmt.Sprintf(" (modified %s, %s ago)", f.FileModified.Format("2006-01-02 15:04:05"), snapshot.FormatAge(max(age, 0)))
	}
	return c
}

func accountCheck(f Facts) Check {
	c := Check{Check: "account", Status: OK}
	switch {
	case f.Account == "":
		c.Detail = "The account folder is not part of the path (--file was used), so there is nothing to choose between."
	case len(f.OtherAccounts) == 0:
		c.Detail = fmt.Sprintf("Account %q is the only one with a dump.", f.Account)
	case f.AccountPinned:
		c.Detail = fmt.Sprintf("Account %q is pinned (--account / WOW_ACCOUNT). Other accounts with a dump: %s.", f.Account, strings.Join(f.OtherAccounts, ", "))
	default:
		c.Status = Warn
		c.Detail = fmt.Sprintf("Several accounts have a dump; the most recently modified one is used: %q. Others: %s.", f.Account, strings.Join(f.OtherAccounts, ", "))
		c.Fix = "If characters are missing, pin the right account with --account <folder name> (or the WOW_ACCOUNT variable) and restart the server."
	}
	return c
}

func dumpAge(d dump) Check {
	c := Check{Check: "dump age", Status: OK}
	if !d.HasDumpedAt {
		c.Status, c.Detail = Warn, "The dump has no time stamp (a very old addon)."
		c.Fix = "Update the addon (copy it into AddOns) and type /reload in game."
		return c
	}
	c.Detail = fmt.Sprintf("The snapshot is %s old.", snapshot.FormatAge(d.Age))
	if d.Stale {
		c.Status = Warn
		c.Detail += " That is older than the stale threshold, so advice may be out of date."
		c.Fix = "Type /mcp reload in game (or click Save on the WowMcp panel); the game writes the file only on /reload, logout and exit."
	}
	return c
}

func addonVersion(d dump) Check {
	c := Check{Check: "addon version", Status: OK}
	if d.AddonVersion == "" {
		c.Status, c.Detail = Warn, "The dump does not say which addon version wrote it (a very old addon)."
		c.Fix = "Copy the current addon into AddOns and type /reload in game."
		return c
	}
	c.Detail = fmt.Sprintf("Addon %s, the server expects %s.", d.AddonVersion, addon.ExpectedVersion)
	if d.OutdatedAddon {
		c.Status = Warn
		c.Fix = "Update the addon: copy the new WowMcp folder into Interface/AddOns (install.ps1 does it) and type /reload in game."
	}
	return c
}

func sectionErrors(d dump) Check {
	c := Check{Check: "section errors", Status: OK, Detail: "The addon reported no errors while collecting sections."}
	if len(d.Errors) == 0 {
		return c
	}
	first := strings.Join(d.Errors[:min(len(d.Errors), 3)], " | ")
	c.Status = Warn
	c.Detail = fmt.Sprintf("The addon reported %d error(s) while collecting sections; the affected sections may be missing or partial. First: %s", len(d.Errors), first)
	c.Fix = "Type /mcp reload again; if the errors stay, report them (the full list is in wow_status as addonErrors) together with the addon and client versions."
	return c
}

func dumpSpeed(d dump) Check {
	c := Check{Check: "dump speed", Status: OK}
	if !d.HasDumpMs {
		c.Detail = "The dump does not report its collection time (addon older than 0.5.0)."
		return c
	}
	c.Detail = fmt.Sprintf("Collecting the dump took %g ms.", d.DumpMs)
	if d.SlowDump {
		c.Status, c.Detail = Warn, d.SlowDumpMessage+"."
		c.Fix = "Turn off the slowest section you do not need with /mcp off <section> (see disabled sections below), then /mcp reload."
	}
	return c
}

func disabledSections(d dump) Check {
	c := Check{Check: "disabled sections", Status: OK, Detail: "No section is turned off in the addon."}
	if len(d.Disabled) == 0 {
		return c
	}
	c.Status, c.Detail = Warn, d.DisabledNote
	c.Fix = "Turn a section back on with /mcp on <section> and type /mcp reload."
	return c
}

// optionalWindows are visit-based sections that are expected to be missing for many players.
var optionalWindows = map[string]bool{"guild": true, "inspect": true}

func windowSections(d dump) Check {
	c := Check{Check: "window sections", Status: OK, Detail: "Every window-based section has been captured at least once."}
	var missing []string
	for _, w := range d.Windows {
		if w.Captured || optionalWindows[w.Section] {
			continue
		}
		missing = append(missing, fmt.Sprintf("%s (open: %v; read by %v)", w.Section, w.Open, w.Tool))
	}
	if len(missing) == 0 {
		return c
	}
	c.Status = Warn
	c.Detail = fmt.Sprintf("%d section(s) were never captured because their window was not opened with the addon enabled: %s", len(missing), strings.Join(missing, "; "))
	c.Fix = "Open the windows you want answers about (vendor, trainer, bank, mailbox, auction house, professions...) once, then type /mcp reload."
	return c
}

func unloadedDump(d dump, f Facts) Check {
	c := Check{Check: "unloaded dump", Status: OK}
	switch {
	case d.Unloaded:
		c.Status = Fail
		c.Detail = "This dump was written while exiting the game, when the client no longer had character data (0 gold, empty bags and spellbook); it does not describe the character."
		c.Fix = "Log in with the character and type /mcp reload. Addon 0.7.1 and newer no longer saves such dumps."
		return c
	case f.HistorySkippedUnloaded > 0:
		c.Detail = fmt.Sprintf("The history skips %d dump(s) written on game exit (empty client); the files stay on disk but do not count in progress or changes.", f.HistorySkippedUnloaded)
	default:
		c.Detail = "The dump holds a loaded character."
	}
	if d.KeptDumpOnLogout {
		c.Detail += " The addon kept the previous dump when the game was last closed."
	}
	return c
}

func clientLanguage(d dump) Check {
	c := Check{Check: "client language", Status: OK}
	if d.Locale == "" {
		c.Detail = "The dump does not report the client locale (addon older than 0.9.0); an English client is assumed."
		return c
	}
	c.Detail = "Client locale " + d.Locale + "."
	if d.EnglishOnlyNote != "" {
		c.Status, c.Detail = Warn, "Client locale "+d.Locale+". "+d.EnglishOnlyNote
		c.Fix = "Nothing to repair: item, stat and profession logic uses IDs and works. Treat results of the name-based heuristics as incomplete; names stay as the client shows them."
	}
	return c
}

// classBuffIDs warns about spell IDs of the addon's class buff groups that the client names as something else.
func classBuffIDs(d dump) Check {
	c := Check{Check: "class buff spell IDs", Status: OK, Detail: "The class buff spells the addon asked about have the names the group rules expect, or the client is not English and the names cannot be compared."}
	if len(d.BadClassBuffIDs) == 0 {
		return c
	}
	var items []string
	for _, m := range d.BadClassBuffIDs {
		items = append(items, fmt.Sprintf("%d is %q (%s group)", m.ID, m.Name, m.Group))
	}
	c.Status = Warn
	c.Detail = "These spell IDs of the addon's class buff list are other spells in this game version and were ignored: " + strings.Join(items, ", ") + "."
	c.Fix = "Nothing to repair on your side: the missing-buff warnings skip those spells. Report the IDs (wow_buffs shows them as groupIdMismatches) so the addon's list can be corrected."
	return c
}

func historyCheck(f Facts) Check {
	c := Check{Check: "history", Status: OK}
	switch {
	case f.HistoryDir == "":
		c.Status = Warn
		c.Detail = "History is disabled, so wow_changes, wow_progress, wow_gold_history and the timeline have nothing to compare."
		c.Fix = "Start the server with a --history-dir (the default is the history folder next to the executable)."
	case f.HistoryError != "":
		c.Status = Fail
		c.Detail = fmt.Sprintf("The history directory %s cannot be written: %s", f.HistoryDir, f.HistoryError)
		c.Fix = "Make the directory writable or point --history-dir to another folder outside the game directory."
	default:
		c.Detail = f.HistoryDir + " is writable."
		if f.Character != "" {
			c.Detail = fmt.Sprintf("%s is writable; %d usable snapshot(s) of %s.", f.HistoryDir, f.HistoryEntries, f.Character)
			if f.HistoryEntries < 2 {
				c.Detail += " Progress tools need at least two snapshots: they appear after more /mcp reload cycles."
			}
		}
	}
	return c
}

func goalsCheck(f Facts) Check {
	c := Check{Check: "goals file", Status: OK}
	if f.GoalsFile == "" {
		c.Detail = "Goals are disabled (no --goals-file); wow_goals and wow_goal_set return an error."
		return c
	}
	c.Detail = "Goals are stored in " + f.GoalsFile + "."
	return c
}
