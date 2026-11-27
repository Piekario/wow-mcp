-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section lookups: the items and spells the player asked the client about with /mcp item and /mcp spell
-- (LookupItem.lua, LookupSpell.lua). Newest first, at most MAX entries, one entry per item or spell.
-- Kept between dumps like the other window sections.

local _, ns = ...
local Snapshot = ns.Snapshot

local MAX = 30

local Lookups = {}
ns.Lookups = Lookups

-- Stores the entry (kind, id, name, query, at and the item or spell fields) in the current dump. The same item or
-- spell asked again replaces its earlier entry. False when the player turned the section off.
function Lookups.add(entry)
    if Snapshot.isDisabled("lookups") then return false end
    local current = Snapshot.current()
    local list = type(current.lookups) == "table" and current.lookups or {}
    for i = #list, 1, -1 do
        local old = list[i]
        if type(old) ~= "table" or (old.kind == entry.kind and old.id == entry.id) then table.remove(list, i) end
    end
    table.insert(list, 1, entry)
    while #list > MAX do table.remove(list) end
    current.lookups = list
    Snapshot.windowCaptured("lookups")
    return true
end

-- One chat line saying what was stored.
function Lookups.confirmation(entry)
    local what = entry.kind .. " " .. tostring(entry.id) .. " " .. tostring(entry.name)
    if entry.rank then what = what .. " (" .. entry.rank .. ")" end
    return what .. " saved for the assistant. It reaches the disk on /mcp reload."
end

Snapshot.addWindowSection("lookups")
