-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Beast Lore observations (Hunter): when the target or the mouseover unit carries the Beast Lore aura, the lines of
-- its unit tooltip are saved with the beast (Beasts.record, source "beastLore"). The Forever tooltip format is not
-- confirmed, so the lines stay raw and the server reads the abilities from them. The addon casts nothing.

local _, ns = ...
local plain, callFirst = ns.Compat.plain, ns.Compat.callFirst
local Beasts = ns.Beasts

local BeastLore = {}
ns.BeastLore = BeastLore

BeastLore.SPELL_ID = 1462 -- Beast Lore, the same in every client language
BeastLore.MAX_LINES = 30
local MAX_AURAS = 40
local FILTERS = { "HARMFUL", "HELPFUL" }

local lastLines = {} -- npcId -> the joined lines saved last in this session, so a repeated event writes nothing

-- Scans the auras one by one: C_UnitAuras.GetAuraDataByIndex (a table), otherwise UnitAura (spell ID is return 10).
local function auraByIndex(unit)
    for _, filter in ipairs(FILTERS) do
        for i = 1, MAX_AURAS do
            local result = callFirst({ "C_UnitAuras.GetAuraDataByIndex", "UnitAura" }, unit, i, filter)
            if not result or result[1] == nil then break end
            local spellID = result[10]
            if type(result[1]) == "table" then spellID = result[1].spellId end
            if plain(spellID) == BeastLore.SPELL_ID then return true end
        end
    end
    return false
end

-- The direct lookup when the client has it (one call; UNIT_AURA fires often), otherwise the scan.
function BeastLore.hasAura(unit)
    local result = callFirst({ "C_UnitAuras.GetUnitAuraBySpellID" }, unit, BeastLore.SPELL_ID)
    if result then return type(result[1]) == "table" end
    return auraByIndex(unit)
end

-- The unit's tooltip lines (C_TooltipInfo.GetUnit), at most MAX_LINES; nil when the client gives none.
function BeastLore.tooltipLines(unit)
    local lines = ns.Compat.tooltipFor("GetUnit", unit)
    if not lines then return nil end
    while #lines > BeastLore.MAX_LINES do table.remove(lines) end
    return lines
end

local function observe(unit)
    if not (Beasts.isHunter() and ns.Compat.firstValue({ "UnitExists" }, unit)) then return end
    local info = Beasts.unitInfo(unit)
    if not (info and BeastLore.hasAura(unit)) then return end
    local lines = BeastLore.tooltipLines(unit)
    if not lines then return end
    local joined = table.concat(lines, "\n")
    if lastLines[info.npcId] == joined then return end
    lastLines[info.npcId] = joined
    Beasts.record(info, "beastLore", { loreLines = lines })
end

-- For /mcp probe: whether the current target has the aura and, when it has, its tooltip lines.
function BeastLore.probeTarget()
    if not ns.Compat.firstValue({ "UnitExists" }, "target") then return { target = false } end
    local info = Beasts.unitInfo("target")
    local out = { target = true, npcId = info and info.npcId, name = ns.Compat.firstValue({ "UnitName" }, "target") }
    out.hasAura = BeastLore.hasAura("target")
    if out.hasAura then out.lines = BeastLore.tooltipLines("target") end
    return out
end

local function guarded(handler)
    return ns.Snapshot.whenEnabled("beasts", function(...) pcall(handler, ...) end)
end

ns.Events.on("PLAYER_TARGET_CHANGED", guarded(function() observe("target") end))
ns.Events.on("UPDATE_MOUSEOVER_UNIT", guarded(function() observe("mouseover") end))
-- UNIT_AURA(unit, ...): the aura lands on the target after the cast, while it is already targeted.
ns.Events.on("UNIT_AURA", guarded(function(unit)
    if plain(unit) == "target" then observe("target") end
end))
