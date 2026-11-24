-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section titles: the titles the character has earned and the one shown now (GetNumTitles, IsTitleKnown,
-- GetTitleName, GetCurrentTitle). Absent when the client has no title API. The addon never changes the title.

local _, ns = ...
local Compat = ns.Compat
local firstValue = Compat.firstValue

local MAX_TITLES = 300

-- GetTitleName pads the name with a space on the side where the character name goes ("Private ", " the Explorer").
local function titleName(titleID)
    local name = firstValue({ "GetTitleName" }, titleID)
    if type(name) ~= "string" then return nil end
    name = name:gsub("^%s+", ""):gsub("%s+$", "")
    return Compat.pick(name ~= "", name, nil)
end

local function collectTitles()
    local count = tonumber(firstValue({ "GetNumTitles" }))
    if not count or not Compat.resolve("IsTitleKnown") then return nil end
    local titles = { total = count, known = {} }
    for titleID = 1, math.min(count, MAX_TITLES) do
        if firstValue({ "IsTitleKnown" }, titleID) then
            local name = titleName(titleID)
            if name then titles.known[#titles.known + 1] = { id = titleID, name = name } end
        end
    end
    local current = tonumber(firstValue({ "GetCurrentTitle" }))
    if current and current > 0 then
        titles.currentID = current
        titles.current = titleName(current)
    end
    return titles
end

ns.Snapshot.addSection("titles", collectTitles)
