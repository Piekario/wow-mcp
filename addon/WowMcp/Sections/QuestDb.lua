-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Sections questDb (where the quests of the log and of the next guide steps start and end, from the installed
-- quest database, see QuestDb.lua) and questLookups (answers to /mcp quest <id or name>, kept between dumps).
-- Both are absent when the player has no quest database addon.

local _, ns = ...
local QuestDb, Guide, Snapshot = ns.QuestDb, ns.Guide, ns.Snapshot

local MAX_QUESTS = 60
local MAX_LOOKUPS = 10

local function collectQuestDb()
    if not QuestDb.available() then return nil end
    local section = { source = QuestDb.source(), quests = {} }
    local unknown, seen = {}, {}
    local function add(questID, mark)
        if type(questID) ~= "number" then return end
        local quest = seen[questID]
        if quest == nil then
            if #section.quests >= MAX_QUESTS then return end
            quest = QuestDb.quest(questID) or false
            seen[questID] = quest
            if quest then
                section.quests[#section.quests + 1] = quest
            else
                unknown[#unknown + 1] = questID
            end
        end
        if quest then quest[mark] = true end
    end
    local logIDs = {}
    for questID in pairs(Guide.logState()) do logIDs[#logIDs + 1] = questID end
    table.sort(logIDs)
    for _, questID in ipairs(logIDs) do add(questID, "inLog") end
    for _, questID in ipairs(Guide.pendingQuestIDs()) do add(questID, "inGuide") end
    if #unknown > 0 then section.unknownQuestIDs = unknown end
    return section
end

local QuestLookup = {}
ns.QuestLookup = QuestLookup

-- Looks the text up and stores the answer in the current dump (newest first). Returns the stored entry, or nil
-- when there is no quest database.
function QuestLookup.run(query)
    local result = QuestDb.lookup(query)
    if not result then return nil end
    local entry = { query = tostring(query), at = time(), quests = result.quests, npcs = result.npcs }
    if Snapshot.isDisabled("questLookups") then return entry end
    local current = Snapshot.current()
    local lookups = type(current.questLookups) == "table" and current.questLookups or {}
    for i = #lookups, 1, -1 do
        if type(lookups[i]) ~= "table" or lookups[i].query == entry.query then table.remove(lookups, i) end
    end
    table.insert(lookups, 1, entry)
    while #lookups > MAX_LOOKUPS do table.remove(lookups) end
    current.questLookups = lookups
    Snapshot.windowCaptured("questLookups")
    return entry
end

local function plainPlace(_, _, text) return text end

-- "Falla Sagewind, The Barrens 48.2, 32.8" for a quest giver, finisher or objective target. `placeText`
-- (location, title, text) may turn the "zone x, y" part into something else, such as a waypoint link; `quest`
-- names the quest in that title.
function QuestLookup.describeTarget(target, placeText, quest)
    placeText = placeText or plainPlace
    local name = target.name or (target.kind .. " " .. tostring(target.id))
    local parts = { name }
    local where = type(target.locations) == "table" and target.locations[1]
    if where then
        local place = where.zone or "unknown zone"
        if where.inInstance then
            place = place .. " (inside the instance)"
        elseif where.x and where.y then
            local title = (quest and quest.name) and (name .. " (" .. quest.name .. ")") or name
            place = placeText(where, title, place .. " " .. where.x .. ", " .. where.y)
        end
        parts[#parts + 1] = place
        if (target.spawnCount or 1) > 1 then parts[#parts + 1] = target.spawnCount .. " spawn points" end
    elseif target.kind == "item" then
        parts[#parts + 1] = "item"
    end
    return table.concat(parts, ", ")
end

local function describeTargets(targets, placeText, quest)
    if type(targets) ~= "table" or #targets == 0 then return "not in the database" end
    local parts = {}
    for _, target in ipairs(targets) do parts[#parts + 1] = QuestLookup.describeTarget(target, placeText, quest) end
    return table.concat(parts, "; ")
end

local function hasCoordinates(targets)
    for _, target in ipairs(type(targets) == "table" and targets or {}) do
        local where = type(target.locations) == "table" and target.locations[1]
        if where and where.x and where.y and not where.inInstance then return true end
    end
    return false
end

-- Chat lines for a lookup entry, and whether any of them shows coordinates. `placeText` as in describeTarget.
function QuestLookup.lines(entry, placeText)
    local lines, coordinates = {}, false
    for _, quest in ipairs(entry.quests) do
        lines[#lines + 1] = ("%s (quest %d, level %s)"):format(quest.name or "?", quest.id, tostring(quest.level or "?"))
        lines[#lines + 1] = "  starts: " .. describeTargets(quest.startedBy, placeText, quest)
        if type(quest.objectives) == "table" and #quest.objectives > 0 then
            lines[#lines + 1] = "  objectives: " .. describeTargets(quest.objectives, placeText, quest)
        end
        lines[#lines + 1] = "  ends: " .. describeTargets(quest.finishedBy, placeText, quest)
        coordinates = coordinates or hasCoordinates(quest.startedBy) or hasCoordinates(quest.objectives)
            or hasCoordinates(quest.finishedBy)
    end
    for _, npc in ipairs(entry.npcs) do
        lines[#lines + 1] = "NPC " .. QuestLookup.describeTarget(npc, placeText)
        coordinates = coordinates or hasCoordinates({ npc })
    end
    if #lines == 0 then lines[1] = "Nothing found for \"" .. entry.query .. "\" (use the exact quest or NPC name, or an ID)." end
    return lines, coordinates
end

Snapshot.addSection("questDb", collectQuestDb)
Snapshot.addWindowSection("questLookups")
