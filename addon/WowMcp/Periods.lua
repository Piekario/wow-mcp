-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Loot and XP periods carried between dumps.

local _, ns = ...
local Store, Snapshot = ns.Store, ns.Snapshot

local Periods = {}
ns.Periods = Periods

-- Whether the saved dump ends with a skipped logout dump (the client already had empty data):
-- then the disk holds an old dump, and the loot and XP period continues from it.
function Periods.continues(saved)
    return type(saved) == "table" and type(saved.lastSkippedLogoutAt) == "number"
        and type(saved.dumpedAt) == "number" and saved.lastSkippedLogoutAt >= saved.dumpedAt
end

-- Closed periods. The addon never assumes the server read a dump: the game may save twice
-- (/mcp reload, then logout) before the server looks at the file, and the second save would otherwise
-- replace the first period's loot and XP. So when a login closes a period that has content, the period
-- (its fields plus until = the dumpedAt of the save that closed it) is appended to the section's
-- "periods" list, oldest first, and carried on. The list is bounded by count and by age; empty periods
-- are not stored.
Periods.MAX_PERIODS = 20
Periods.MAX_PERIOD_AGE = 7 * 24 * 3600

-- The carried periods that are still within the bounds at time now: tables with a numeric until not older
-- than MAX_PERIOD_AGE, at most MAX_PERIODS of the newest. Returns a new list (possibly empty).
function Periods.kept(periods, now)
    local kept = {}
    if type(periods) ~= "table" then return kept end
    for _, period in ipairs(periods) do
        local ended = type(period) == "table" and period["until"]
        if type(ended) == "number" and now - ended <= Periods.MAX_PERIOD_AGE then kept[#kept + 1] = period end
    end
    while #kept > Periods.MAX_PERIODS do table.remove(kept, 1) end
    return kept
end

-- Closes the saved section's period at time ended: returns the carried list with the closed period appended
-- when hasContent(section) says it holds anything, bounded by keptPeriods; nil when the list is empty.
function Periods.close(section, ended, hasContent, now)
    if type(section) ~= "table" then return nil end
    local periods = section.periods
    section.periods = nil
    if type(section.since) == "number" and hasContent(section) then
        section["until"] = ended
        periods = type(periods) == "table" and periods or {}
        periods[#periods + 1] = section
    end
    local kept = Periods.kept(periods, now)
    if #kept == 0 then return nil end
    return kept
end

-- After login (and after /reload) starts a new period of section name: the previous dump is already
-- on disk, so the section starts from zero with since = its dumpedAt. fresh(since) creates an empty section,
-- and the closed period goes to the section's periods list when hasContent(section) is true.
-- When the previous logout dump was skipped, the period continues (with its periods). Returns since or nil.
function Periods.start(name, fresh, hasContent)
    local saved = Store.savedCurrentCharacter()
    local previous = saved and type(saved[name]) == "table" and saved[name] or nil
    if Periods.continues(saved) and previous and type(previous.since) == "number" then
        local kept = Periods.kept(previous.periods, time())
        previous.periods = #kept > 0 and kept or nil
        return previous.since
    end
    local since = (saved and type(saved.dumpedAt) == "number" and saved.dumpedAt) or time()
    if Snapshot.isDisabled(name) then
        if saved then saved[name] = nil end
        return since
    end
    local section = fresh(since)
    section.periods = Periods.close(previous, since, hasContent, time())
    Snapshot.current()[name] = section
    return since
end
