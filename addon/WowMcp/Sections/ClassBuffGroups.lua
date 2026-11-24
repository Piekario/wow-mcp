-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section classBuffGroups: the player's class buff groups (ClassBuffs.lua) with the name the client gives each spell,
-- in the client's language. A spell ID the client does not know is left out; a class without groups has no section.

local _, ns = ...
local plain, firstValue = ns.Compat.plain, ns.Compat.firstValue

local function spellName(id)
    local name = firstValue({ "C_Spell.GetSpellName", "GetSpellInfo" }, id)
    if type(name) == "string" and name ~= "" then return name end
    return nil
end

local function groupSpells(ids)
    local spells = {}
    for _, id in ipairs(ids) do
        local name = spellName(id)
        if name then spells[#spells + 1] = { id = id, name = name } end
    end
    return spells
end

local function collectGroups()
    local _, classToken = UnitClass("player")
    local groups = {}
    for _, group in ipairs(ns.ClassBuffs.groupsFor(plain(classToken)) or {}) do
        local spells = groupSpells(group.ids)
        if #spells > 0 then groups[#groups + 1] = { label = group.label, spells = spells } end
    end
    if #groups == 0 then return nil end
    return groups
end

ns.Snapshot.addSection("classBuffGroups", collectGroups)
