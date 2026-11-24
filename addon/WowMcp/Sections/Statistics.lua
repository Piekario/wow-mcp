-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section statistics: the counters the game keeps for the character (the Statistics tab of the achievement window),
-- by category. A statistic is an achievement entry in a category of GetStatisticsCategoryList and its value comes from
-- GetStatistic(id) as display text ("--" = never happened, skipped). Absent when the client has no such API.
-- Category and statistic names are read once per session; the values on every dump.

local _, ns = ...
local Compat = ns.Compat
local plain, firstValue = Compat.plain, Compat.firstValue
local Categories = ns.AchievementCategories

local MAX_STATISTICS = 400

local layout -- { categories = { {id, name, parentID, statistics = { {id, name} }} }, notListed }

local function buildLayout()
    local built, count = { categories = {}, notListed = 0 }, 0
    for _, category in ipairs(Categories.read(GetStatisticsCategoryList)) do
        local statistics = {}
        for index = 1, Categories.count(category.id) do
            if count >= MAX_STATISTICS then
                built.notListed = built.notListed + 1
            else
                local info = Categories.achievement(category.id, index)
                if info then
                    statistics[#statistics + 1] = { id = info.id, name = info.name }
                    count = count + 1
                end
            end
        end
        built.categories[#built.categories + 1] = {
            id = category.id, name = category.title, parentID = category.parentID, statistics = statistics,
        }
    end
    return built
end

-- Money values carry coin icons; they become "5g 20s 3c".
local function valueText(value)
    value = plain(value)
    if type(value) == "number" then return tostring(value) end
    if type(value) ~= "string" then return nil end
    value = value:gsub("|T[^|]*GoldIcon[^|]*|t", "g"):gsub("|T[^|]*SilverIcon[^|]*|t", "s"):gsub("|T[^|]*CopperIcon[^|]*|t", "c")
    value = Compat.stripMarkup(value):gsub("^%s+", ""):gsub("%s+$", "")
    if value == "" or value == "--" then return nil end
    return value
end

local function collectStatistics()
    if not (Compat.resolve("GetStatisticsCategoryList") and Compat.resolve("GetStatistic")) then return nil end
    layout = layout or buildLayout()
    if #layout.categories == 0 then return nil end
    local result, empty = { categories = {} }, 0
    for _, category in ipairs(layout.categories) do
        local list = {}
        for _, statistic in ipairs(category.statistics) do
            local value = valueText(firstValue({ "GetStatistic" }, statistic.id))
            if value then
                list[#list + 1] = { id = statistic.id, name = statistic.name, value = value }
            else
                empty = empty + 1
            end
        end
        if #list > 0 then
            result.categories[#result.categories + 1] = {
                id = category.id, name = category.name, parentID = category.parentID, list = list,
            }
        end
    end
    result.empty = empty
    result.notListed = Compat.pick(layout.notListed > 0, layout.notListed, nil)
    return result
end

ns.Snapshot.addSection("statistics", collectStatistics)
