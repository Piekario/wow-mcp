-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Sections quests (quest log with objectives and rewards) and completedQuests (IDs of completed quests).

local _, ns = ...
local plain, callFirst = ns.Compat.plain, ns.Compat.callFirst

local function modernObjectives(questID)
    local objectives = C_QuestLog.GetQuestObjectives(questID)
    if not objectives then return nil end
    local out = {}
    for _, o in ipairs(objectives) do
        out[#out + 1] = {
            text = plain(o.text),
            done = o.finished and true or false,
            have = plain(o.numFulfilled),
            need = plain(o.numRequired),
        }
    end
    return out
end

-- Quest rewards (QuestRewards); a reward read error does not spoil the rest of the quest.
local function questRewards(read, key)
    local ok, rewards = pcall(read, key)
    if ok then return rewards end
    return nil
end

local function positiveNumber(value)
    if type(value) == "number" and value > 0 then return value end
    return nil
end

local function modernQuest(info, zone, zoneMapID)
    local q = {
        id = plain(info.questID),
        title = plain(info.title),
        level = plain(info.level),
        zone = zone,
        zoneMapID = zoneMapID,
        suggestedGroup = positiveNumber(plain(info.suggestedGroup)),
    }
    if info.questID then
        if C_QuestLog.IsComplete then
            q.complete = C_QuestLog.IsComplete(info.questID) and true or false
        end
        if C_QuestLog.IsFailed then
            q.failed = C_QuestLog.IsFailed(info.questID) and true or nil
        end
        if C_QuestLog.GetQuestObjectives then
            q.objectives = modernObjectives(info.questID)
        end
        q.rewards = questRewards(ns.QuestRewards.fromLog, info.questID)
    end
    return q
end

-- Map ID of a log header that is a zone name (nil for other headers); the same number in every client language.
local function headerMapID(zoneIDs, header)
    if type(header) ~= "string" then return nil end
    return zoneIDs[header]
end

-- New quest log API (C_QuestLog). A log header is usually the zone name; zoneMapID is that zone's UiMapID.
local function modernQuestLog()
    local quests = {}
    local header
    local okIDs, zoneIDs = pcall(ns.Maps.zoneIDsByName)
    if not okIDs then zoneIDs = {} end
    for i = 1, C_QuestLog.GetNumQuestLogEntries() or 0 do
        local info = C_QuestLog.GetInfo(i)
        if info then
            if info.isHeader then
                header = plain(info.title)
            elseif not info.isHidden then
                quests[#quests + 1] = modernQuest(info, header, headerMapID(zoneIDs, header))
            end
        end
    end
    return quests
end

local function legacyObjectives(questIndex)
    local objectives = {}
    for j = 1, GetNumQuestLeaderBoards(questIndex) or 0 do
        local text, _, finished = GetQuestLogLeaderBoard(j, questIndex)
        objectives[#objectives + 1] = { text = plain(text), done = finished and true or false }
    end
    return objectives
end

-- Old quest log API.
local function legacyQuestLog()
    local quests = {}
    local header
    for i = 1, GetNumQuestLogEntries() or 0 do
        local title, level, _, isHeader, _, isComplete, _, questID = GetQuestLogTitle(i)
        title = plain(title)
        if isHeader then
            header = title
        elseif title then
            local q = {
                id = plain(questID),
                title = title,
                level = plain(level),
                zone = header,
                complete = (isComplete == 1 or isComplete == true),
                failed = (isComplete == -1) or nil,
            }
            if GetNumQuestLeaderBoards and GetQuestLogLeaderBoard then
                q.objectives = legacyObjectives(i)
            end
            q.rewards = questRewards(ns.QuestRewards.fromLogEntry, i)
            quests[#quests + 1] = q
        end
    end
    return quests
end

local function collectQuests()
    if C_QuestLog and C_QuestLog.GetNumQuestLogEntries and C_QuestLog.GetInfo then
        return modernQuestLog()
    end
    if GetNumQuestLogEntries and GetQuestLogTitle then
        return legacyQuestLog()
    end
    return {}
end

-- IDs of all completed quests. Lets us check whether the character has already done
-- the prerequisite quest for another quest.
local function collectCompletedQuests()
    local results = callFirst({ "C_QuestLog.GetAllCompletedQuestIDs" })
    if not results or type(results[1]) ~= "table" then return nil end
    local ids = {}
    for _, id in ipairs(results[1]) do
        if type(id) == "number" then ids[#ids + 1] = id end
    end
    table.sort(ids)
    return ids
end

ns.Snapshot.addSection("quests", collectQuests)
ns.Snapshot.addSection("completedQuests", collectCompletedQuests)
