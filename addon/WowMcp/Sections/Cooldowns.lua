-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section cooldowns: long cooldowns (over 90 s) of abilities, items in bags
-- (Hearthstone) and professions (e.g. transmutes). readyAt is unix time, so the server
-- computes how much is left, even long after the dump.

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue
local Items = ns.Items

-- Shorter cooldowns expire before anyone reads the dump.
local MIN_DURATION = 90

-- Cooldown read: the new API returns a table, the old one three values. Result: start, duration.
local function cooldownTimes(results)
    if not results then return nil end
    local start, duration
    if type(results[1]) == "table" then
        start, duration = plain(results[1].startTime), plain(results[1].duration)
    else
        start, duration = plain(results[1]), plain(results[2])
    end
    if type(start) ~= "number" or type(duration) ~= "number" then return nil end
    return start, duration
end

-- Set of cooldowns without repeated names. GetTime() is the game clock, time() is unix time.
local function newCooldownList()
    local nowGame, nowUnix = GetTime(), time()
    local list, seen = {}, {}
    local function add(name, kind, start, duration, id)
        if not name or seen[name] or not start or start <= 0 or duration <= MIN_DURATION then return end
        local remaining = start + duration - nowGame
        if remaining <= 0 then return end
        seen[name] = true
        list[#list + 1] = {
            name = name, kind = kind, id = id,
            remainingSeconds = math.floor(remaining + 0.5),
            readyAt = nowUnix + math.floor(remaining + 0.5),
        }
    end
    return list, add
end

local function addSpellCooldowns(add)
    for _, spell in ipairs(ns.Spellbook.knownSpells()) do
        if not spell.passive then
            local start, duration = cooldownTimes(callFirst({ "C_Spell.GetSpellCooldown", "GetSpellCooldown" }, spell.id or spell.name))
            add(spell.name, "spell", start, duration, spell.id)
        end
    end
end

local function addBagItemCooldowns(add)
    for bag = 0, Items.lastEquippedBag() do
        for slot = 1, Items.containerSlotCount(bag) do
            local start, duration = cooldownTimes(callFirst({ "C_Container.GetContainerItemCooldown", "GetContainerItemCooldown" }, bag, slot))
            if start and start > 0 then
                local link = firstValue({ "C_Container.GetContainerItemLink", "GetContainerItemLink" }, bag, slot)
                add(Items.nameFromLink(link), "item", start, duration, Items.idFromLink(link))
            end
        end
    end
end

local function collectCooldowns()
    if not GetTime then return nil end
    local cooldowns, add = newCooldownList()
    addSpellCooldowns(add)
    addBagItemCooldowns(add)
    table.sort(cooldowns, function(a, b) return a.remainingSeconds < b.remainingSeconds end)
    return cooldowns
end

ns.Snapshot.addSection("cooldowns", collectCooldowns)
