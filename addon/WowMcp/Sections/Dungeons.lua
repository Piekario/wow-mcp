-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section dungeons: what the client itself says about dungeons (the group finder) and
-- saved lockouts. Level ranges from the game take precedence over the server's table.

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue

local MAX_ENTRIES = 200

-- A plain number or nil (clients that return fewer values leave the field out).
local function numberOrNil(value)
    value = plain(value)
    if type(value) == "number" then return value end
    return nil
end

local function lfgCategoryName(categoryID)
    local catInfo = callFirst({ "C_LFGList.GetLfgCategoryInfo", "C_LFGList.GetCategoryInfo" }, categoryID)
    return catInfo and (type(catInfo[1]) == "table" and plain(catInfo[1].name) or plain(catInfo[1]))
end

local function lfgActivity(activityID, categoryName)
    local info = callFirst({ "C_LFGList.GetActivityInfoTable" }, activityID)
    info = info and info[1]
    if not (type(info) == "table" and plain(info.fullName)) then return nil end
    return {
        id = activityID, name = info.fullName, category = categoryName,
        categoryID = plain(info.categoryID), groupID = plain(info.groupFinderActivityGroupID),
        minLevel = plain(info.minLevel), maxLevel = plain(info.maxLevel),
        minLevelSuggestion = plain(info.minLevelSuggestion),
        maxLevelSuggestion = plain(info.maxLevelSuggestion),
        maxPlayers = plain(info.maxNumPlayers),
    }
end

-- Appends the activities of one category; returns true when the list reached the limit.
local function appendCategoryActivities(activities, categoryID)
    local categoryName = lfgCategoryName(categoryID)
    local acts = callFirst({ "C_LFGList.GetAvailableActivities" }, categoryID)
    for _, activityID in ipairs((acts and type(acts[1]) == "table") and acts[1] or {}) do
        activities[#activities + 1] = lfgActivity(activityID, categoryName)
        if #activities >= MAX_ENTRIES then return true end
    end
    return false
end

-- C_LFGList: categories (Dungeons, Raids, ...) and activities with a level range.
-- The limit applies to the whole list, not to a single category.
local function lfgListActivities()
    local cats = callFirst({ "C_LFGList.GetAvailableCategories" })
    if not (cats and type(cats[1]) == "table") then return nil end
    local activities = {}
    for _, categoryID in ipairs(cats[1]) do
        if appendCategoryActivities(activities, categoryID) then break end
    end
    if #activities == 0 then return nil end
    return activities
end

-- Older LFG (GetLFGDungeonInfo) with the ID list from GetLFDChoiceOrder.
local function lfgDungeons()
    local order = callFirst({ "GetLFDChoiceOrder" })
    if not (order and type(order[1]) == "table" and ns.Compat.resolve("GetLFGDungeonInfo")) then return nil end
    local dungeons = {}
    for _, id in ipairs(order[1]) do
        local r = callFirst({ "GetLFGDungeonInfo" }, id)
        if r and plain(r[1]) then
            -- name, typeID, subtypeID, minLevel, maxLevel, recLevel, minRecLevel, maxRecLevel, expansionLevel, groupID,
            -- texture, difficulty, maxPlayers (13), description, isHoliday, bonusRep, minPlayers, isTimewalker,
            -- name2, minGearLevel, isScalingDungeon, lfgMapID (22: the instance's map ID, same in every language)
            dungeons[#dungeons + 1] = {
                id = id, name = r[1], minLevel = plain(r[4]), maxLevel = plain(r[5]),
                recLevel = plain(r[6]), minRecLevel = plain(r[7]), maxRecLevel = plain(r[8]),
                maxPlayers = plain(r[13]), typeID = numberOrNil(r[2]), groupID = numberOrNil(r[10]),
                mapID = numberOrNil(r[22]),
            }
        end
        if #dungeons >= MAX_ENTRIES then break end
    end
    if #dungeons == 0 then return nil end
    return dungeons
end

-- Saved instance lockouts (raid ID).
local function savedLockouts()
    local saved = firstValue({ "GetNumSavedInstances" })
    if not (type(saved) == "number" and saved > 0) then return nil end
    local lockouts = {}
    for i = 1, saved do
        local r = callFirst({ "GetSavedInstanceInfo" }, i)
        if r and plain(r[1]) then
            -- name, id, reset, difficultyID, locked, extended, mostsig, isRaid, maxPlayers, difficultyName,
            -- numEncounters, encounterProgress, extendDisabled, instanceID (14: the instance's map ID)
            local reset = plain(r[3])
            lockouts[#lockouts + 1] = {
                name = r[1], id = plain(r[2]), instanceID = numberOrNil(r[14]), difficultyID = numberOrNil(r[4]),
                resetAt = type(reset) == "number" and (time() + reset) or nil,
                locked = r[5] and true or false, raid = r[8] and true or nil, maxPlayers = plain(r[9]),
                difficulty = plain(r[10]), encounters = plain(r[11]), killed = plain(r[12]),
            }
        end
    end
    return lockouts
end

local function collectDungeons()
    local out = {}
    out.activities = lfgListActivities()
    out.lfg = lfgDungeons()
    out.lockouts = savedLockouts()
    if next(out) == nil then return nil end
    return out
end

ns.Snapshot.addSection("dungeons", collectDungeons)
