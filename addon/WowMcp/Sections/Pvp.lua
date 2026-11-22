-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section pvp: honor statistics, if the client supplies them readily (honorable kills, honor,
-- rank). Every field is optional; with none supplied the section is absent from the dump.

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue

local function number(value)
    if type(value) == "number" then return value end
    return nil
end

local function collectPvp()
    local pvp = {}

    -- GetPVPLifetimeStats: honorableKills, dishonorableKills, highestRank.
    local lifetime = callFirst({ "GetPVPLifetimeStats" })
    if lifetime then
        pvp.honorableKills = number(plain(lifetime[1]))
        pvp.dishonorableKills = number(plain(lifetime[2]))
        pvp.highestRank = number(plain(lifetime[3]))
    end

    pvp.honor = number(firstValue({ "UnitHonor" }, "player"))
    pvp.honorMax = number(firstValue({ "UnitHonorMax" }, "player"))
    pvp.honorLevel = number(firstValue({ "UnitHonorLevel" }, "player"))

    -- Rank: raw number from UnitPVPRank (0 = no rank), rank name and number from GetPVPRankInfo.
    -- Clients without ranks return nothing.
    local raw = number(firstValue({ "UnitPVPRank" }, "player"))
    if raw and raw > 0 then
        pvp.rank = raw
        local info = callFirst({ "GetPVPRankInfo" }, raw, "player")
        local name = info and plain(info[1])
        if type(name) == "string" and name ~= "" then pvp.rankName = name end
        pvp.rank = number(info and plain(info[2])) or raw
    end

    if next(pvp) == nil then return nil end
    return pvp
end

ns.Snapshot.addSection("pvp", collectPvp)
