-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- QuestDb: where quests start and end, read from the quest database of another addon the player has installed
-- (QuestieDB, the database Questie uses; global LibQuestieDB). The client itself has no API for "which NPC ends
-- this quest and where does it stand", and the quest text is often out of date, so this is the only source that
-- matches what the player sees on the map. WowMcp ships no quest data of its own: without QuestieDB every
-- function here returns nil.
--
-- Only reads (entity Get / IdsByName, its zone support data and C_Map.GetAreaInfo); never registers corrections or
-- changes that addon.
-- Every read is wrapped in pcall: a change in the other addon costs a field, not the dump.

local _, ns = ...
local plain = ns.Compat.plain
local round1 = ns.round1

local QuestDb = {}
ns.QuestDb = QuestDb

QuestDb.MAX_LOCATIONS = 3  -- sample coordinates per NPC or object
QuestDb.MAX_ZONES = 4      -- zone names listed for something that spawns in many zones
QuestDb.MAX_TARGETS = 4    -- starters, finishers or objective targets listed per quest
QuestDb.MAX_MATCHES = 8    -- quests or NPCs returned for one name search

local function lib()
    local db = _G.LibQuestieDB
    if type(db) ~= "table" then return nil end
    if type(db.Quest) ~= "table" or type(db.Npc) ~= "table" then return nil end
    return db
end
QuestDb.lib = lib

function QuestDb.available()
    return lib() ~= nil
end

-- Who supplies the data: goes into the dump so an answer can name its source.
function QuestDb.source()
    local db = lib()
    if not db then return nil end
    local version
    local getMeta = (C_AddOns and C_AddOns.GetAddOnMetadata) or _G.GetAddOnMetadata
    if type(getMeta) == "function" and type(db.addonName) == "string" then
        local ok, value = pcall(getMeta, db.addonName, "Version")
        if ok then version = plain(value) end
    end
    return { name = "QuestieDB", version = version, contract = plain(db.contractVersion), readMode = plain(db.readMode) }
end

-- One field of an entity ("Quest", "Npc", "Object", "Item"), or nil.
function QuestDb.field(entityName, id, key)
    local db = lib()
    local entity = db and db[entityName]
    if type(entity) ~= "table" or type(entity.Get) ~= "function" then return nil end
    local ok, value = pcall(entity.Get, id, key)
    if ok then return value end
    return nil
end
local field = QuestDb.field

local function exists(entityName, id)
    local db = lib()
    local entity = db and db[entityName]
    if type(entity) ~= "table" or type(entity.Exists) ~= "function" then return false end
    local ok, value = pcall(entity.Exists, id)
    return ok and value == true
end

-- The database stores some strings as byte strings; both arrive as Lua strings.
local function text(value)
    if type(value) == "string" and value ~= "" then return value end
    return nil
end

local function positive(value)
    if type(value) == "number" and value > 0 then return value end
    return nil
end

-- Zone (area) name in the client's language, or nil.
function QuestDb.areaName(areaID)
    if type(areaID) ~= "number" or areaID <= 0 then return nil end
    if not (C_Map and C_Map.GetAreaInfo) then return nil end
    local ok, name = pcall(C_Map.GetAreaInfo, areaID)
    if ok then return text(plain(name)) end
    return nil
end

-- The database's zone support data (area ID -> UI map ID) arrives as Lua source text; it is decoded once, in an empty
-- environment, the same way Questie decodes it. The override table wins, as in Questie, so a map ID here is the map
-- Questie draws the point on.
local uiMapIDs

local function decodeMapTable(source)
    if type(source) == "table" then return source end
    if type(source) ~= "string" or type(loadstring) ~= "function" then return {} end
    local chunk = loadstring(source)
    if not chunk then return {} end
    if type(setfenv) == "function" then setfenv(chunk, {}) end
    local ok, decoded = pcall(chunk)
    return (ok and type(decoded) == "table") and decoded or {}
end

local function loadUiMapIDs()
    local db = lib()
    local support = db and db.Support
    if type(support) ~= "table" or type(support.Get) ~= "function" then return {} end
    local ok, zones = pcall(support.Get, "ZoneDB")
    local private = ok and type(zones) == "table" and zones.private
    if type(private) ~= "table" then return {} end
    local merged = {}
    for areaID, mapID in pairs(decodeMapTable(private.areaIdToUiMapId)) do merged[areaID] = mapID end
    for areaID, mapID in pairs(decodeMapTable(private.areaIdToUiMapIdOverride)) do merged[areaID] = mapID end
    return merged
end

-- UI map ID (what map pins and waypoints use) of a database area ID, or nil (unknown area, or no database).
function QuestDb.uiMapID(areaID)
    if type(areaID) ~= "number" or not QuestDb.available() then return nil end
    if not (uiMapIDs and next(uiMapIDs)) then uiMapIDs = loadUiMapIDs() end
    return positive(uiMapIDs[areaID])
end

local function sortedKeys(map)
    local keys = {}
    for key in pairs(map) do
        if type(key) == "number" then keys[#keys + 1] = key end
    end
    table.sort(keys)
    return keys
end

-- Spawns ({ [areaID] = { {x, y}, ... } }) as a short description: sample coordinates (the entity's main zone
-- first), the number of spawn points and the zones. A point of (-1, -1) means "inside this instance".
local function describeSpawns(target, spawns, mainArea)
    if type(spawns) ~= "table" then return end
    local areas = sortedKeys(spawns)
    if type(mainArea) == "number" and spawns[mainArea] then
        table.sort(areas, function(a, b)
            if (a == mainArea) ~= (b == mainArea) then return a == mainArea end
            return a < b
        end)
    end
    local locations, zones, count = {}, {}, 0
    for _, areaID in ipairs(areas) do
        local points = spawns[areaID]
        if type(points) == "table" then
            local zone = QuestDb.areaName(areaID)
            if #zones < QuestDb.MAX_ZONES then zones[#zones + 1] = zone or ("area " .. areaID) end
            for _, point in ipairs(points) do
                count = count + 1
                if #locations < QuestDb.MAX_LOCATIONS and type(point) == "table" then
                    local x, y = point[1], point[2]
                    if x == -1 and y == -1 then
                        locations[#locations + 1] = { zone = zone, areaID = areaID, inInstance = true }
                    elseif type(x) == "number" and type(y) == "number" then
                        locations[#locations + 1] = { zone = zone, areaID = areaID, x = round1(x), y = round1(y) }
                    end
                end
            end
        end
    end
    if count == 0 then return end
    target.locations = locations
    target.spawnCount = count
    if #areas > 1 then
        target.zones = zones
        target.zoneCount = #areas
    end
end

function QuestDb.npc(id)
    if type(id) ~= "number" or not exists("Npc", id) then return nil end
    local target = {
        kind = "npc",
        id = id,
        name = text(field("Npc", id, "name")),
        title = text(field("Npc", id, "subName")),
        friendlyTo = text(field("Npc", id, "friendlyToFaction")),
        minLevel = positive(field("Npc", id, "minLevel")),
        maxLevel = positive(field("Npc", id, "maxLevel")),
    }
    describeSpawns(target, field("Npc", id, "spawns"), field("Npc", id, "zoneID"))
    return target
end

function QuestDb.object(id)
    if type(id) ~= "number" or not exists("Object", id) then return nil end
    local target = { kind = "object", id = id, name = text(field("Object", id, "name")) }
    describeSpawns(target, field("Object", id, "spawns"), field("Object", id, "zoneID"))
    return target
end

-- An item is only named: where it drops is a separate (large) table we do not copy.
function QuestDb.item(id)
    if type(id) ~= "number" or not exists("Item", id) then return nil end
    return { kind = "item", id = id, name = text(field("Item", id, "name")) }
end

local DESCRIBE = { QuestDb.npc, QuestDb.object, QuestDb.item }

-- Quest givers / finishers: { npcIds?, objectIds?, itemIds? } -> list of targets (at most MAX_TARGETS).
local function describeGivers(givers)
    local out, total = {}, 0
    if type(givers) ~= "table" then return out, total end
    for group = 1, 3 do
        local ids = givers[group]
        if type(ids) == "table" then
            for _, id in ipairs(ids) do
                total = total + 1
                if #out < QuestDb.MAX_TARGETS then
                    out[#out + 1] = DESCRIBE[group](id) or { kind = ({ "npc", "object", "item" })[group], id = id }
                end
            end
        end
    end
    return out, total
end

-- Objectives: { creatures, objects, items, ... }, each a list of { id, text? } -> list of targets with the
-- objective text. Reputation, kill credit and spell groups are left out.
local function describeObjectives(objectives)
    local out = {}
    if type(objectives) ~= "table" then return out end
    for group = 1, 3 do
        local entries = objectives[group]
        if type(entries) == "table" then
            for _, entry in ipairs(entries) do
                if #out >= QuestDb.MAX_TARGETS then return out end
                local id = type(entry) == "table" and entry[1] or nil
                if type(id) == "number" then
                    local target = DESCRIBE[group](id) or { kind = ({ "npc", "object", "item" })[group], id = id }
                    target.text = text(entry[2])
                    out[#out + 1] = target
                end
            end
        end
    end
    return out
end

local function nonEmpty(list)
    if type(list) == "table" and #list > 0 then return list end
    return nil
end

local function questRef(id)
    if type(id) ~= "number" or id <= 0 or not exists("Quest", id) then return nil end
    return { id = id, name = text(field("Quest", id, "name")) }
end

local function objectiveText(lines)
    if type(lines) ~= "table" then return nil end
    local parts = {}
    for _, line in ipairs(lines) do
        if text(line) then parts[#parts + 1] = line end
    end
    if #parts == 0 then return nil end
    return table.concat(parts, " ")
end

-- Everything the database says about where a quest starts, what it needs and where it ends; nil when the
-- database is missing or does not know the quest.
function QuestDb.quest(id)
    if type(id) ~= "number" or not exists("Quest", id) then return nil end
    local quest = {
        id = id,
        name = text(field("Quest", id, "name")),
        level = positive(field("Quest", id, "questLevel")),
        requiredLevel = positive(field("Quest", id, "requiredLevel")),
        objectivesText = objectiveText(field("Quest", id, "objectivesText")),
        nextQuest = questRef(field("Quest", id, "nextQuestInChain")),
    }
    local zoneOrSort = field("Quest", id, "zoneOrSort")
    if positive(zoneOrSort) then
        quest.zone = QuestDb.areaName(zoneOrSort)
        quest.zoneAreaID = zoneOrSort
    end
    local starters, starterCount = describeGivers(field("Quest", id, "startedBy"))
    local finishers, finisherCount = describeGivers(field("Quest", id, "finishedBy"))
    quest.startedBy = nonEmpty(starters)
    quest.finishedBy = nonEmpty(finishers)
    if starterCount > #starters then quest.startedByCount = starterCount end
    if finisherCount > #finishers then quest.finishedByCount = finisherCount end
    quest.objectives = nonEmpty(describeObjectives(field("Quest", id, "objectives")))
    return quest
end

-- IDs whose name equals the given text exactly (the database's own name index; the first call builds it).
local function idsByName(entityName, name)
    local db = lib()
    local entity = db and db[entityName]
    if type(entity) ~= "table" or type(entity.IdsByName) ~= "function" then return {} end
    local ok, ids = pcall(entity.IdsByName, name)
    if ok and type(ids) == "table" then return ids end
    return {}
end

-- Capitalizes the first letter of every word, so "the glowing shard" finds "The Glowing Shard".
local function titleCase(name)
    return (name:gsub("(%a)([%w']*)", function(first, rest) return first:upper() .. rest end))
end

local function nameCandidates(name)
    local candidates = { name }
    local cased = titleCase(name)
    if cased ~= name then candidates[#candidates + 1] = cased end
    return candidates
end

-- Looks a quest or NPC up by ID or by exact name (also with every word capitalized). Returns
-- { quests = { ... }, npcs = { ... } }, both possibly empty; nil when there is no database.
function QuestDb.lookup(query)
    if not QuestDb.available() then return nil end
    local result = { quests = {}, npcs = {} }
    query = tostring(query or ""):match("^%s*(.-)%s*$")
    local id = tonumber(query)
    if id then
        result.quests[1] = QuestDb.quest(id)
        if #result.quests == 0 then result.npcs[1] = QuestDb.npc(id) end
        return result
    end
    if query == "" then return result end
    local seen = {}
    for _, name in ipairs(nameCandidates(query)) do
        for _, questID in ipairs(idsByName("Quest", name)) do
            if not seen["q" .. questID] and #result.quests < QuestDb.MAX_MATCHES then
                seen["q" .. questID] = true
                result.quests[#result.quests + 1] = QuestDb.quest(questID)
            end
        end
        for _, npcID in ipairs(idsByName("Npc", name)) do
            if not seen["n" .. npcID] and #result.npcs < QuestDb.MAX_MATCHES then
                seen["n" .. npcID] = true
                result.npcs[#result.npcs + 1] = QuestDb.npc(npcID)
            end
        end
    end
    return result
end
