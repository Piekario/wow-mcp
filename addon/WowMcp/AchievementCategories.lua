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
