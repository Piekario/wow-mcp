-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section reputation: factions with reputation level (no group headers).

local _, ns = ...
local plain = ns.Compat.plain

-- Reputation level name (e.g. "Friendly"); when the client does not know it, just the number.
local function standingLabel(reaction)
    return reaction and _G["FACTION_STANDING_LABEL" .. reaction] or reaction
end

local function modernFactions()
    local factions = {}
    for index = 1, C_Reputation.GetNumFactions() or 0 do
        local ok, data = pcall(C_Reputation.GetFactionDataByIndex, index)
        if ok and type(data) == "table" and data.name and not data.isHeader then
            factions[#factions + 1] = {
                name = data.name,
                standing = standingLabel(plain(data.reaction)),
                value = plain(data.currentStanding),
                floor = plain(data.currentReactionThreshold),
                ceiling = plain(data.nextReactionThreshold),
            }
        end
    end
    return factions
end

local function legacyFactions()
    local factions = {}
    for index = 1, GetNumFactions() or 0 do
        local name, _, standingID, floorValue, ceilingValue, earned, _, _, isHeader = GetFactionInfo(index)
        if name and not isHeader then
            factions[#factions + 1] = {
                name = name,
                standing = standingLabel(standingID),
                value = earned, floor = floorValue, ceiling = ceilingValue,
            }
        end
    end
    return factions
end

local function collectReputation()
    if C_Reputation and C_Reputation.GetNumFactions and C_Reputation.GetFactionDataByIndex then
        return modernFactions()
    end
    if GetNumFactions and GetFactionInfo then
        return legacyFactions()
    end
    return {}
end

ns.Snapshot.addSection("reputation", collectReputation)
