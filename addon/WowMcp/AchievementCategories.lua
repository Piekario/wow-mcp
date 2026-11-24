-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Achievement categories and the achievements in them, read the same way for the legacy (challenges), achievements
-- and statistics sections. A statistic is an achievement entry in a category of GetStatisticsCategoryList.
-- Every call is guarded: a client without the achievement API gives empty results.

local _, ns = ...
local Compat = ns.Compat
local plain = Compat.plain

local AchievementCategories = {}
ns.AchievementCategories = AchievementCategories

-- GetAchievementInfo gives the year as years since 2000 (26 = 2026).
local function isoDate(month, day, year)
    month, day, year = tonumber(plain(month)), tonumber(plain(day)), tonumber(plain(year))
    if not (month and day and year) or month < 1 or day < 1 then return nil end
    if year < 100 then year = year + 2000 end
    return string.format("%04d-%02d-%02d", year, month, day)
end

-- {id, title, parentID} for each category ID that listCategories returns (GetCategoryList or
-- GetStatisticsCategoryList), in the client's order. parentID is absent for a top-level category (the API says -1).
function AchievementCategories.read(listCategories)
    local list = {}
    if type(listCategories) ~= "function" or type(GetCategoryInfo) ~= "function" then return list end
    local ok, ids = pcall(listCategories)
    if not ok or type(ids) ~= "table" then return list end
    for _, categoryID in ipairs(ids) do
        local id = plain(categoryID)
        if id then
            local okInfo, title, parentID = pcall(GetCategoryInfo, id)
            if okInfo then
                parentID = tonumber(plain(parentID))
                list[#list + 1] = { id = id, title = plain(title), parentID = Compat.pick(parentID and parentID > 0, parentID, nil) }
            end
        end
    end
    return list
end

-- Total and completed achievements of a category, including the later steps of a series the UI hides.
function AchievementCategories.count(categoryID)
    if type(GetCategoryNumAchievements) ~= "function" then return 0, nil end
    local ok, total, completed = pcall(GetCategoryNumAchievements, categoryID, true)
    if not ok then return 0, nil end
    return tonumber(plain(total)) or 0, tonumber(plain(completed))
end

-- The achievement at index in a category: id, name, points, completed, completedOn (YYYY-MM-DD), description,
-- reward, earnedByMe; nil when the client gives none.
function AchievementCategories.achievement(categoryID, index)
    if type(GetAchievementInfo) ~= "function" then return nil end
    local ok, id, name, points, completed, month, day, year, description, _, _, rewardText, _, earnedByMe =
        pcall(GetAchievementInfo, categoryID, index)
    id = ok and plain(id)
    if not id then return nil end
    rewardText = plain(rewardText)
    completed = completed and true or false
    return {
        id = id, name = plain(name), points = plain(points), completed = completed,
        completedOn = Compat.pick(completed, isoDate(month, day, year), nil),
        description = plain(description), reward = Compat.pick(rewardText == "", nil, rewardText),
        earnedByMe = Compat.pick(plain(earnedByMe) == nil, nil, earnedByMe and true or false),
    }
end

-- The Legacy challenges: achievements that grant Legacy Points. Two rules, both on English client text:
--   * the achievement category title (or its parent's) contains "Legacy";
--   * the reward text contains "Legacy Point" (real client 2026-11-24: "Earn 1 Legacy Point." on achievements in
--     ordinary categories such as Classes > Warlock; no category is named Legacy there).
-- The achievements section reads every achievement once per dump and leaves its result in ns.Achievements.latest,
-- so this reads from there; without it (section off or no achievement API) only the category rule can be applied.

local MAX_CHALLENGES = 200
local TITLE_RULE = "achievement category title contains Legacy"
local REWARD_RULE = "reward text contains Legacy Point"

local function mentions(text, fragment)
    return type(text) == "string" and text:lower():find(fragment, 1, true) ~= nil
end

local function copyCriteria(criteria)
    if type(criteria) ~= "table" then return nil end
    local out = {}
    for _, c in ipairs(criteria) do
        out[#out + 1] = { text = c.text, completed = c.completed, quantity = c.quantity, required = c.required }
    end
    return out
end

local function entryOf(a, title)
    return {
        id = a.id, category = title, categoryID = a.category, name = a.name, points = a.points,
        completed = a.completed, completedOn = a.completedOn, earnedByMe = a.earnedByMe,
        description = a.description, reward = a.reward, criteria = copyCriteria(a.criteria),
    }
end

local function matchedBy(byTitle, byReward)
    local rules = {}
    if byTitle then rules[#rules + 1] = TITLE_RULE end
    if byReward then rules[#rules + 1] = REWARD_RULE end
    if #rules == 0 then return "none matched (rules: " .. TITLE_RULE .. "; " .. REWARD_RULE .. ")" end
    return table.concat(rules, " + ")
end

-- Titles by category ID and whether the category (or its parent) is a Legacy one.
local function categoryTable(categories)
    local titles, parents, legacy = {}, {}, {}
    for _, c in ipairs(categories) do
        titles[c.id], parents[c.id] = c.name or c.title, c.parentID
    end
    for id, title in pairs(titles) do
        if mentions(title, "legacy") or (parents[id] and mentions(titles[parents[id]], "legacy")) then legacy[id] = true end
    end
    return titles, legacy
end

local function fromAchievements(result)
    local titles, legacyCategory = categoryTable(result.categories or {})
    local list, byTitle, byReward = {}, false, false
    for _, a in ipairs(result.list or {}) do
        local inCategory, hasReward = legacyCategory[a.category], mentions(a.reward, "legacy point")
        if (inCategory or hasReward) and #list < MAX_CHALLENGES then
            byTitle, byReward = byTitle or inCategory, byReward or hasReward
            list[#list + 1] = entryOf(a, titles[a.category])
        end
    end
    return { matchedBy = matchedBy(byTitle, byReward), list = list }
end

-- Without the achievements section: the category rule only, reading the achievements of the matching categories.
local function fromCategories()
    local read = AchievementCategories.read(GetCategoryList)
    local mapped = {}
    for _, c in ipairs(read) do mapped[#mapped + 1] = { id = c.id, name = c.title, parentID = c.parentID } end
    local titles, legacyCategory = categoryTable(mapped)
    local list = {}
    for _, c in ipairs(read) do
        if legacyCategory[c.id] then
            for index = 1, AchievementCategories.count(c.id) do
                local a = AchievementCategories.achievement(c.id, index)
                if a and #list < MAX_CHALLENGES then
                    a.category = c.id
                    list[#list + 1] = entryOf(a, titles[c.id])
                end
            end
        end
    end
    return { matchedBy = matchedBy(#list > 0, false) .. " (the achievements section was not available)", list = list }
end

function AchievementCategories.legacyChallenges()
    local latest = ns.Achievements and ns.Achievements.latest
    if latest then return fromAchievements(latest) end
    if not (GetCategoryList and GetCategoryInfo and GetCategoryNumAchievements and GetAchievementInfo) then return nil end
    return fromCategories()
end
