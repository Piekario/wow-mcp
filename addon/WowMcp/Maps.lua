-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Maps: zone map IDs (UiMapID) by zone name, so the dump carries a number that is the same in every client language
-- (quest log headers and flight points only have the translated zone name). Only reads map data (C_Map).

local _, ns = ...
local plain = ns.Compat.plain

local Maps = {}
ns.Maps = Maps

local CONTINENT_MAP_TYPE = 2 -- Enum.UIMapType.Continent
local ZONE_MAP_TYPE = 3      -- Enum.UIMapType.Zone
local MAX_PARENT_STEPS = 10

function Maps.mapType(name, default)
    local types = Enum and Enum.UIMapType
    return (types and types[name]) or default
end

-- The player's map ID and position on it in percent (as on the world map); nil for what the client does not supply.
function Maps.playerPosition()
    if not (C_Map and C_Map.GetBestMapForUnit) then return nil end
    local mapID = plain(C_Map.GetBestMapForUnit("player"))
    if not (mapID and C_Map.GetPlayerMapPosition) then return mapID end
    local pos = C_Map.GetPlayerMapPosition(mapID, "player")
    if not (pos and pos.GetXY) then return mapID end
    local rawX, rawY = pos:GetXY()
    local x, y = plain(rawX), plain(rawY)
    if not (x and y) then return mapID end
    return mapID, ns.round1(x * 100), ns.round1(y * 100)
end

-- Continent the character is on (the player's map and its parents); nil when the client does not supply it.
function Maps.playerContinent()
    if not (C_Map and C_Map.GetBestMapForUnit and C_Map.GetMapInfo) then return nil end
    local mapID = plain(C_Map.GetBestMapForUnit("player"))
    for _ = 1, MAX_PARENT_STEPS do
        if not mapID then return nil end
        local ok, info = pcall(C_Map.GetMapInfo, mapID)
        if not (ok and type(info) == "table") then return nil end
        if plain(info.mapType) == Maps.mapType("Continent", CONTINENT_MAP_TYPE) then return mapID end
        mapID = plain(info.parentMapID)
    end
    return nil
end

-- Children of a map of the given type (all descendants); an empty list when the client has none.
local function children(mapID, typeName, default)
    if not (mapID and C_Map and C_Map.GetMapChildrenInfo) then return {} end
    local ok, list = pcall(C_Map.GetMapChildrenInfo, mapID, Maps.mapType(typeName, default), true)
    return (ok and type(list) == "table") and list or {}
end

local function addZones(names, mapID)
    for _, zone in ipairs(children(mapID, "Zone", ZONE_MAP_TYPE)) do
        local id, name = plain(zone.mapID), plain(zone.name)
        if id and type(name) == "string" and name ~= "" and names[name] == nil then names[name] = id end
    end
end

-- Zone name -> map ID for the zones of the world map's continents and of the player's continent. Built on demand
-- (a few C_Map calls) and returned as a plain table; empty when the client has no map API.
function Maps.zoneIDsByName()
    local names = {}
    local ok, worldID = pcall(function() return C_Map and C_Map.GetFallbackWorldMapID and plain(C_Map.GetFallbackWorldMapID()) end)
    local continents = {}
    if ok and worldID then
        for _, continent in ipairs(children(worldID, "Continent", CONTINENT_MAP_TYPE)) do
            continents[#continents + 1] = plain(continent.mapID)
        end
    end
    local own = Maps.playerContinent()
    if own then continents[#continents + 1] = own end
    for _, continentID in ipairs(continents) do addZones(names, continentID) end
    return names
end

