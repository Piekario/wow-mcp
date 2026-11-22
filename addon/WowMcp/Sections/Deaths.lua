-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section deaths: the character's death counter and the last few (zone, level, time).
-- Counts the PLAYER_DEAD event; the data lives in the character's saved dump and passes
-- through subsequent dumps like window sections. The addon only records that you died.

local _, ns = ...
local firstValue = ns.Compat.firstValue

local MAX_RECENT = 20

local function currentDeaths(current)
    local deaths = current.deaths
    if type(deaths) ~= "table" then deaths = {} end
    if type(deaths.count) ~= "number" then deaths.count = 0 end
    if type(deaths.recent) ~= "table" then deaths.recent = {} end
    return deaths
end

local function deathPlace()
    return {
        zone = firstValue({ "GetRealZoneText" }),
        subZone = firstValue({ "GetSubZoneText" }),
        level = firstValue({ "UnitLevel" }, "player"),
        at = time(),
    }
end

local function trimOldest(recent)
    while #recent > MAX_RECENT do table.remove(recent, 1) end
end

local function onPlayerDead()
    local deaths = currentDeaths(ns.Snapshot.current())
    deaths.count = deaths.count + 1
    deaths.recent[#deaths.recent + 1] = deathPlace()
    trimOldest(deaths.recent)
    ns.Snapshot.current().deaths = deaths
end

ns.Snapshot.addWindowSection("deaths", nil, nil, { quiet = true })
ns.Events.on("PLAYER_DEAD", ns.Snapshot.whenEnabled("deaths", function() pcall(onPlayerDead) end))
