-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Detects the dump of a client that has already unloaded the character.

local _, ns = ...

local LogoutGuard = {}
ns.LogoutGuard = LogoutGuard

local function isEmptyList(value)
    return type(value) == "table" and next(value) == nil
end

local function hasNoBagSpace(bags)
    if type(bags) ~= "table" then return false end
    for _, bag in ipairs(bags) do
        if type(bag) == "table" and type(bag.size) == "number" and bag.size > 0 then return false end
    end
    return true
end

-- The four sections whose emptiness says the client has unloaded the character. The guard reads them
-- even when the player turned them off with /mcp off (see guardView), so it never depends on that choice.
local GUARD_SECTIONS = { "stats", "spells", "equipment", "bags" }

-- How many empty-client signs the dump has.
function LogoutGuard.unloadedSignals(snapshot)
    local signals = 0
    if type(snapshot.stats) == "table" and not (type(snapshot.stats.healthMax) == "number" and snapshot.stats.healthMax > 0) then
        signals = signals + 1
    end
    if isEmptyList(snapshot.spells) then signals = signals + 1 end
    if isEmptyList(snapshot.equipment) then signals = signals + 1 end
    if hasNoBagSpace(snapshot.bags) then signals = signals + 1 end
    return signals
end

-- The sections the guard looks at: the dump's own, and for a disabled one a fresh read that is only used for the
-- check (never stored in the dump). A section that errored simply gives no sign, as before.
-- collectorOf(name) returns a section's collect function; isDisabled(name) says whether /mcp off turned it off.
local function guardView(snapshot, collectorOf, isDisabled)
    local view = {}
    for _, name in ipairs(GUARD_SECTIONS) do
        view[name] = snapshot[name]
        local collect = collectorOf(name)
        if view[name] == nil and collect and isDisabled(name) then
            local ok, result = pcall(collect)
            if ok then view[name] = result end
        end
    end
    return view
end

-- On a real exit from the game (and logout to the menu) PLAYER_LOGOUT arrives once the client has
-- already unloaded the character: zero health, no spells, equipment or bags. Two signs are enough;
-- a real character has none (it always has a backpack, health and Attack in the spellbook).
-- Independent of /mcp off: a disabled sign section is read for the check only.
function LogoutGuard.looksUnloaded(snapshot, collectorOf, isDisabled)
    return LogoutGuard.unloadedSignals(guardView(snapshot, collectorOf, isDisabled)) >= 2
end
