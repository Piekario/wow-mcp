-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Beasts observed by the account's Hunters, keyed by creature (NPC) ID. No game API says which beast has which ability
-- rank, so the addon learns it from what the player does: taming (BeastTaming.lua) and Beast Lore (BeastLore.lua).
-- One entry per creature keeps the latest of each kind of data: abilities from taming, raw tooltip lines from Beast Lore.

local _, ns = ...
local plain, firstValue = ns.Compat.plain, ns.Compat.firstValue

local Beasts = {}
ns.Beasts = Beasts

Beasts.MAX_ENTRIES = 300

function Beasts.isHunter()
    local _, classToken = UnitClass("player")
    return plain(classToken) == "HUNTER"
end

local CREATURE_KINDS = { Creature = true, Vehicle = true }
local PET_KINDS = { Pet = true }

local function idFromGuid(guid, kinds)
    guid = plain(guid)
    if type(guid) ~= "string" then return nil end
    local kind, npcID = guid:match("^(%a+)%-%d+%-%d+%-%d+%-%d+%-(%d+)%-")
    if not kinds[kind] then return nil end
    return tonumber(npcID)
end

-- Creature ID from a unit GUID: "Creature-0-<server>-<instance>-<zone>-<npcID>-<spawn>" (also "Vehicle-"); nil for
-- players, pets and secret GUIDs.
function Beasts.npcIdFromGuid(guid)
    return idFromGuid(guid, CREATURE_KINDS)
end

-- Creature ID of the Hunter's pet: its GUID is "Pet-0-<server>-<instance>-<zone>-<npcID>-<id>" in mainline clients.
-- Not confirmed in Forever; nil when the GUID has another form.
function Beasts.petNpcId()
    return idFromGuid(firstValue({ "UnitGUID" }, "pet"), PET_KINDS)
end

-- Who the unit is: npcId, name, level and family; nil when it is no creature or its GUID cannot be read.
function Beasts.unitInfo(unit)
    local npcId = Beasts.npcIdFromGuid(firstValue({ "UnitGUID" }, unit))
    if not npcId then return nil end
    return {
        npcId = npcId,
        name = firstValue({ "UnitName" }, unit),
        level = firstValue({ "UnitLevel" }, unit),
        family = firstValue({ "UnitCreatureFamily" }, unit),
    }
end

-- Where the player stands now.
local function addPlace(entry)
    entry.zone = firstValue({ "GetRealZoneText" })
    entry.mapId, entry.x, entry.y = ns.Maps.playerPosition()
end

local function trimOldest(store)
    local count, oldestKey, oldestAt = 0, nil, nil
    for key, entry in pairs(store) do
        count = count + 1
        local at = type(entry) == "table" and entry.seenAt or 0
        if oldestAt == nil or at < oldestAt then oldestKey, oldestAt = key, at end
    end
    if count > Beasts.MAX_ENTRIES and oldestKey ~= nil then
        store[oldestKey] = nil
        trimOldest(store)
    end
end

-- Stores an observation of the unit described by info (Beasts.unitInfo). source is "tamed" or "beastLore"; fields
-- holds that source's data (abilities or loreLines), which replaces the same field of an earlier observation.
function Beasts.record(info, source, fields)
    local store = ns.Store.beasts()
    local entry = type(store[info.npcId]) == "table" and store[info.npcId] or {}
    entry.npcId, entry.name, entry.level, entry.family = info.npcId, info.name, info.level, info.family
    entry.source, entry.seenAt = source, time()
    addPlace(entry)
    for key, value in pairs(fields) do entry[key] = value end
    store[info.npcId] = entry
    trimOldest(store)
    return entry
end

local function copy(value)
    if type(value) ~= "table" then return value end
    local out = {}
    for key, inner in pairs(value) do out[key] = copy(inner) end
    return out
end

-- A copy of the observations as a list, most recent first (for the dump, which must not share tables with the store).
function Beasts.list()
    local list = {}
    for _, entry in pairs(ns.Store.beasts()) do
        if type(entry) == "table" and type(entry.npcId) == "number" then list[#list + 1] = copy(entry) end
    end
    table.sort(list, function(a, b)
        if (a.seenAt or 0) ~= (b.seenAt or 0) then return (a.seenAt or 0) > (b.seenAt or 0) end
        return a.npcId < b.npcId
    end)
    return list
end
