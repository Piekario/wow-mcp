-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section achievements: achievement points, categories with their counts, and the achievements with completion dates;
-- for incomplete ones the progress of their criteria. Absent when the client has no achievement API.
-- The category layout and completed achievements do not change during a session, so they are read once; later dumps
-- re-read only the incomplete achievements. Nothing here tracks, links or compares an achievement.

local _, ns = ...
local Compat = ns.Compat
local plain, callFirst, firstValue = Compat.plain, Compat.callFirst, Compat.firstValue
local Categories = ns.AchievementCategories

local MAX_ACHIEVEMENTS = 600
local MAX_LISTED_CRITERIA = 8   -- criteria kept per achievement (the missing ones first)
local MAX_READ_CRITERIA = 25    -- criteria read per achievement
local CRITERIA_BUDGET = 3000    -- criteria read per dump over all achievements

ns.Achievements = {}          -- latest: this dump's result, for the Legacy challenges (read after this section)

local layout                    -- { categories, slots = { {categoryID, index, id} }, notListed }; nil = read on the next dump
local completedCache = {}       -- achievement ID -> entry, once completed

local function buildLayout()
    local built = { categories = Categories.read(GetCategoryList), slots = {}, notListed = 0 }
    for _, category in ipairs(built.categories) do
        for index = 1, Categories.count(category.id) do
            if #built.slots < MAX_ACHIEVEMENTS then
                built.slots[#built.slots + 1] = { categoryID = category.id, index = index }
            else
                built.notListed = built.notListed + 1
            end
        end
    end
    return built
end

-- Reads up to MAX_READ_CRITERIA criteria of an incomplete achievement; returns how many were read.
local function readCriteria(entry, budget)
    local count = tonumber(firstValue({ "GetAchievementNumCriteria" }, entry.id))
    if not count or count <= 0 or not Compat.resolve("GetAchievementCriteriaInfo") then return 0 end
    entry.criteriaTotal = count
    local missing, done = {}, {}
    for index = 1, math.min(count, MAX_READ_CRITERIA, budget) do
        local info = callFirst({ "GetAchievementCriteriaInfo" }, entry.id, index)
        if info then
            local criterion = {
                text = plain(info[1]), completed = info[3] and true or false,
                quantity = plain(info[4]), required = plain(info[5]),
            }
            if criterion.completed then done[#done + 1] = criterion else missing[#missing + 1] = criterion end
        end
    end
    entry.criteriaRead, entry.criteriaDone, entry.criteria = #missing + #done, #done, {}
    for _, list in ipairs({ missing, done }) do
        for _, criterion in ipairs(list) do
            if #entry.criteria < MAX_LISTED_CRITERIA then entry.criteria[#entry.criteria + 1] = criterion end
        end
    end
    return entry.criteriaRead
end

local function readSlot(slot)
    local cached = slot.id and completedCache[slot.id]
    if cached then return cached end
    local entry = Categories.achievement(slot.categoryID, slot.index)
    if not entry then return nil end
    if slot.id and slot.id ~= entry.id then layout.stale = true end
    slot.id = entry.id
    entry.category = slot.categoryID
    if entry.completed then completedCache[entry.id] = entry end
    return entry
end

local function readCategories(categories)
    local out = {}
    for _, category in ipairs(categories) do
        local total, completed = Categories.count(category.id)
        out[#out + 1] = { id = category.id, name = category.title, parentID = category.parentID, total = total, completed = completed }
    end
    return out
end

local function collectAchievements()
    ns.Achievements.latest = nil
    if not (Compat.resolve("GetCategoryList") and Compat.resolve("GetAchievementInfo")) then return nil end
    if not layout or layout.stale then layout = buildLayout() end
    if #layout.categories == 0 then return nil end
    local current = layout
    local result = { categories = readCategories(current.categories), list = {} }
    result.points = tonumber(firstValue({ "GetTotalAchievementPoints" }))
    local counts = callFirst({ "GetNumCompletedAchievements" })
    if counts then
        result.total, result.completed = tonumber(plain(counts[1])), tonumber(plain(counts[2]))
    end
    local budget, skipped = CRITERIA_BUDGET, 0
    for _, slot in ipairs(current.slots) do
        local entry = readSlot(slot)
        if entry then
            result.list[#result.list + 1] = entry
            if not entry.completed and budget > 0 then
                budget = budget - readCriteria(entry, budget)
            elseif not entry.completed then
                skipped = skipped + 1
            end
        end
    end
    result.notListed = Compat.pick(current.notListed > 0, current.notListed, nil)
    result.criteriaSkipped = Compat.pick(skipped > 0, skipped, nil)
    ns.Achievements.latest = result
    return result
end

ns.Snapshot.addSection("achievements", collectAchievements)
