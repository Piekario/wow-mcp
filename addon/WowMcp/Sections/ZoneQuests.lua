-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section zoneQuests: every quest the installed quest database files under the current zone and under the zones of
-- the log quests (see QuestZones.lua), with the race and class masks of the character, so the server can tell what is
-- done, what is left and which chains are unfinished. Absent when the player has no quest database addon.

local _, ns = ...
local QuestDb, QuestZones, Guide = ns.QuestDb, ns.QuestZones, ns.Guide
local plain, firstValue = ns.Compat.plain, ns.Compat.firstValue

local MAX_ZONES = 3 -- the current zone and the zones with the most log quests

-- The character's race and class as the database's bit masks, and the race masks of both factions
-- (QuestieDB contract 3 and 4 tables); each part is absent when the database does not offer it.
local function restrictions()
    local db = QuestDb.lib()
    local enum = db and db.Enum
    if type(enum) ~= "table" then return nil, nil end
    local player = {}
    local _, classToken = UnitClass("player")
    local _, _, raceID = UnitRace("player")
    classToken, raceID = plain(classToken), plain(raceID)
    if type(enum.classKeys) == "table" and classToken then player.classMask = plain(enum.classKeys[classToken]) end
    if type(enum.raceMaskById) == "table" and raceID then player.raceMask = plain(enum.raceMaskById[raceID]) end
    local masks = enum.factionRaceMasks
    local factions
    if type(masks) == "table" then
        factions = { Alliance = plain(masks.Alliance), Horde = plain(masks.Horde) }
    end
    if next(player) == nil then player = nil end
    return player, factions
end

-- Areas to list: the current one first, then those with the most log quests (ties by area ID).
local function chooseAreas(current)
    local counts = {}
    for questID in pairs(Guide.logState()) do
        local area = QuestZones.areaOfQuest(questID)
        if area then counts[area] = (counts[area] or 0) + 1 end
    end
    local others = {}
    for area in pairs(counts) do
        if area ~= current then others[#others + 1] = area end
    end
    table.sort(others, function(a, b)
        if counts[a] ~= counts[b] then return counts[a] > counts[b] end
        return a < b
    end)
    local areas = {}
    if current then areas[1] = current end
    for _, area in ipairs(others) do
        if #areas >= MAX_ZONES then break end
        areas[#areas + 1] = area
    end
    return areas, counts
end

local function collectZoneQuests()
    if not QuestDb.available() then return nil end
    local section = { currentZone = firstValue({ "GetRealZoneText" }) }
    if not QuestZones.available() then
        section.noZoneIndex = true
        return section
    end
    section.player, section.factionRaceMasks = restrictions()
    local current = QuestZones.areaOf(section.currentZone)
    local areas, logCounts = chooseAreas(current)
    section.zones = {}
    for _, area in ipairs(areas) do
        local zone = QuestZones.zone(area)
        if zone then
            zone.current = (area == current) or nil
            zone.logQuests = logCounts[area]
            section.zones[#section.zones + 1] = zone
        end
    end
    return section
end

ns.Snapshot.addSection("zoneQuests", collectZoneQuests)
