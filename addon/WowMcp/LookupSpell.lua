-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- /mcp spell <id, spell link or name>: reads one spell from the client (name, rank, description, cast time, cost,
-- range and the tooltip lines) and stores it in the lookups section. A spell whose data the client has not loaded
-- yet is requested (RequestLoadSpellData fills the client cache only) and stored when SPELL_DATA_LOAD_RESULT arrives.

local _, ns = ...
local Lookups, Events, TraitTree = ns.Lookups, ns.Events, ns.TraitTree
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue

local SpellLookup = {}
ns.SpellLookup = SpellLookup

local pending = {} -- spellID -> the query as typed

local function target(query)
    local id = tonumber(query:match("spell:(%d+)")) or tonumber(query:match("^%d+$"))
    return id, id or query
end

-- C_Spell.GetSpellInfo returns a table, the older GetSpellInfo name, rank, icon, castTime, minRange, maxRange, spellID.
local function spellInfo(key)
    local result = callFirst({ "C_Spell.GetSpellInfo", "GetSpellInfo" }, key)
    if not result then return nil end
    local first = result[1]
    if type(first) == "table" then
        return plain(first.name), plain(first.spellID), plain(first.castTime), plain(first.minRange), plain(first.maxRange)
    end
    return plain(first), plain(result[7]), plain(result[4]), plain(result[5]), plain(result[6])
end

local function readCosts(id)
    local result = callFirst({ "C_Spell.GetSpellPowerCost", "GetSpellPowerCost" }, id)
    local costs = {}
    for _, cost in ipairs(result and type(result[1]) == "table" and result[1] or {}) do
        local amount = plain(cost.cost)
        if type(amount) == "number" and amount > 0 then
            costs[#costs + 1] = { power = plain(cost.name), amount = amount }
        end
    end
    return #costs > 0 and costs or nil
end

local function readSpell(key, query)
    local name, id, castTime, minRange, maxRange = spellInfo(key)
    id = id or (type(key) == "number" and key) or nil
    if type(name) ~= "string" or not id then return nil end
    local spell = { kind = "spell", id = id, name = name, query = query, at = time() }
    local rank = firstValue({ "C_Spell.GetSpellSubtext", "GetSpellSubtext" }, id)
    if type(rank) == "string" and rank ~= "" then spell.rank = rank end
    spell.castTimeMs, spell.minRange, spell.maxRange = castTime, minRange, maxRange
    spell.costs = readCosts(id)
    spell.description, spell.descriptionSource = TraitTree.spellDescription(id)
    spell.tooltip = ns.Compat.tooltipFor("GetSpellByID", id)
    if not spell.description then
        spell.descriptionMissing = true
        callFirst({ "C_Spell.RequestLoadSpellData" }, id)
    end
    return spell
end

-- Returns the status and the entry: "saved" (entry), "disabled", "pending" (the client was asked to load the spell),
-- "unknown" (a name the client does not resolve) or "usage".
function SpellLookup.run(query)
    query = tostring(query or ""):match("^%s*(.-)%s*$")
    if query == "" then return "usage" end
    local id, key = target(query)
    local spell = readSpell(key, query)
    if spell then return Lookups.add(spell) and "saved" or "disabled", spell end
    if not id then return "unknown" end
    pending[id] = query
    callFirst({ "C_Spell.RequestLoadSpellData" }, id)
    return "pending"
end

Events.on("SPELL_DATA_LOAD_RESULT", function(spellID, success)
    local query = type(spellID) == "number" and pending[spellID]
    if not query then return end
    pending[spellID] = nil
    local spell = success ~= false and readSpell(spellID, query)
    if not spell then
        ns.chat("The client has no spell " .. spellID .. ". Check the ID.")
    elseif Lookups.add(spell) then
        ns.chat(Lookups.confirmation(spell))
    end
end)
