-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- QuestZones: the quests of one zone, from the installed quest database (see QuestDb.lua), in the compact form of the
-- zoneQuests section. The database has no zone index, so the first request walks every quest ID once (GetAllIds and
-- one field per quest) and keeps an area ID -> quest IDs index for the rest of the session; each zone's list is built
-- once and cached too. Only reads.

local _, ns = ...
local QuestDb, round1 = ns.QuestDb, ns.round1

local QuestZones = {}
ns.QuestZones = QuestZones

QuestZones.MAX_QUESTS = 150 -- quests listed per zone
local MAX_SCANNED = 60000   -- quest IDs the index pass looks at, whatever the database returns
local REPEATABLE_FLAG = 1   -- specialFlags bit 1: repeatable
local STARTER_KINDS = { { "npc", "Npc" }, { "object", "Object" }, { "item", "Item" } }

local index        -- area ID -> ascending quest IDs; nil until the first request
local areaByName   -- zone name in the client's language -> area ID
local zones = {}   -- area ID -> built zone

local function text(value)
    if type(value) == "string" and value ~= "" then return value end
    return nil
end

-- The database returns 0, never nil, for an empty numeric field.
local function number(id, key)
    local value = QuestDb.field("Quest", id, key)
    if type(value) == "number" and value ~= 0 then return value end
    return nil
end

local function positive(id, key)
    local value = number(id, key)
    if value and value > 0 then return value end
    return nil
end

local function idList(value)
    if type(value) ~= "table" then return nil end
    local out = {}
    for _, id in ipairs(value) do
        if type(id) == "number" and id > 0 then out[#out + 1] = id end
    end
    if #out == 0 then return nil end
    return out
end

local function nameAreas(counts)
    for area in pairs(index) do
        local name = QuestDb.areaName(area)
        local known = name and areaByName[name]
        if name and (not known or counts[area] > counts[known]) then areaByName[name] = area end
    end
end

local function buildIndex()
    index, areaByName = {}, {}
    local db = QuestDb.lib()
    local quests = db and db.Quest
    if type(quests) ~= "table" or type(quests.GetAllIds) ~= "function" then return end
    local ok, ids = pcall(quests.GetAllIds)
    if not ok or type(ids) ~= "table" then return end
    local counts = {}
    for i, id in ipairs(ids) do
        if i > MAX_SCANNED then break end
        local area = positive(id, "zoneOrSort")
        if area then
            local list = index[area]
            if not list then
                list = {}
                index[area] = list
            end
            list[#list + 1] = id
            counts[area] = #list
        end
    end
    nameAreas(counts)
end

local function ensureIndex()
    if index == nil then buildIndex() end
    return next(index) ~= nil
end

-- True when the zone index could be built (QuestieDB present and offering GetAllIds).
function QuestZones.available()
    return QuestDb.available() and ensureIndex()
end

-- Area ID of a zone name as the client shows it (GetRealZoneText), or nil.
function QuestZones.areaOf(zoneName)
    if type(zoneName) ~= "string" or not ensureIndex() then return nil end
    return areaByName[zoneName]
end

-- Area ID the database files a quest under (its quest log zone), or nil for class, profession and other sorts.
function QuestZones.areaOfQuest(questID)
    if type(questID) ~= "number" then return nil end
    return positive(questID, "zoneOrSort")
end

-- Where a starter stands: coordinates in this zone when it spawns here, otherwise the name of its first zone.
local function placeStarter(starter, spawns, areaID)
    if type(spawns) ~= "table" then return end
    local here = spawns[areaID]
    local point = type(here) == "table" and here[1]
    if type(point) == "table" and type(point[1]) == "number" and type(point[2]) == "number" and point[1] >= 0 then
        starter.x, starter.y = round1(point[1]), round1(point[2])
        return
    end
    local first
    for area in pairs(spawns) do
        if type(area) == "number" and (first == nil or area < first) then first = area end
    end
    starter.zone = QuestDb.areaName(first)
end

local function describeStarter(kind, entity, id, areaID, cache)
    local key = kind .. id
    if cache[key] == nil then
        local starter = { kind = kind, name = text(QuestDb.field(entity, id, "name")) }
        if kind ~= "item" then placeStarter(starter, QuestDb.field(entity, id, "spawns"), areaID) end
        cache[key] = starter
    end
    return cache[key]
end

-- The first NPC, object or item that starts the quest.
local function starterOf(questID, areaID, cache)
    local givers = QuestDb.field("Quest", questID, "startedBy")
    if type(givers) ~= "table" then return nil end
    for group, kinds in ipairs(STARTER_KINDS) do
        local ids = givers[group]
        if type(ids) == "table" and type(ids[1]) == "number" then
            return describeStarter(kinds[1], kinds[2], ids[1], areaID, cache)
        end
    end
    return nil
end

local function questEntry(id, areaID, cache)
    local quest = {
        id = id,
        name = text(QuestDb.field("Quest", id, "name")),
        level = positive(id, "questLevel"),
        requiredLevel = positive(id, "requiredLevel"),
        maxLevel = positive(id, "requiredMaxLevel"),
        races = positive(id, "requiredRaces"),
        classes = positive(id, "requiredClasses"),
        preAny = idList(QuestDb.field("Quest", id, "preQuestSingle")),
        preAll = idList(QuestDb.field("Quest", id, "preQuestGroup")),
        next = positive(id, "nextQuestInChain"),
        exclusiveTo = idList(QuestDb.field("Quest", id, "exclusiveTo")),
        breadcrumbFor = positive(id, "breadcrumbForQuestId"),
        start = starterOf(id, areaID, cache),
    }
    local flags = positive(id, "specialFlags")
    if flags and flags % (REPEATABLE_FLAG * 2) >= REPEATABLE_FLAG then quest.repeatable = true end
    return quest
end

local function buildZone(areaID)
    local ids = index[areaID] or {}
    local zone = { areaID = areaID, name = QuestDb.areaName(areaID), questCount = #ids, quests = {} }
    local cache = {}
    for i = 1, math.min(#ids, QuestZones.MAX_QUESTS) do
        zone.quests[i] = questEntry(ids[i], areaID, cache)
    end
    if #ids > QuestZones.MAX_QUESTS then zone.truncated = true end
    return zone
end

-- The zone's quests: { areaID, name, questCount, quests = { ... }, truncated? }; built on the first request of the
-- session. Returns a new outer table each time, so the caller may add its own marks.
function QuestZones.zone(areaID)
    if type(areaID) ~= "number" or not ensureIndex() then return nil end
    if zones[areaID] == nil then zones[areaID] = buildZone(areaID) end
    local copy = {}
    for key, value in pairs(zones[areaID]) do copy[key] = value end
    return copy
end
