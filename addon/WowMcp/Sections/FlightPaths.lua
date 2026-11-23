-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section flightPaths (from the window): flight points visible on the flight master's map.
-- Diagnostics: taxiMapID, nodeFields (fields of the first raw node with types)
-- and zoneSources (where the zones came from: node, zoneMap, position). A position that only gives a continent
-- goes to the node's continent field, never to zone.

local _, ns = ...
local plain = ns.Compat.plain

local NODE_STATES = { [0] = "current", [1] = "reachable", [2] = "unreachable" }

-- Node coordinates (0-1 on the continent map): the position field (Vector2D or {x, y}) or x and y on the node itself.
local function nodePosition(node)
    local position = node.position
    if type(position) == "table" and type(position.GetXY) == "function" then
        local ok, x, y = pcall(position.GetXY, position)
        if ok and type(plain(x)) == "number" and type(plain(y)) == "number" then return x, y end
    end
    if type(position) == "table" and type(plain(position.x)) == "number" and type(plain(position.y)) == "number" then
        return position.x, position.y
    end
    if type(plain(node.x)) == "number" and type(plain(node.y)) == "number" then return node.x, node.y end
    return nil
end

local CONTINENT_MAP_TYPE = 2 -- Enum.UIMapType.Continent
local ZONE_MAP_TYPE = 3      -- Enum.UIMapType.Zone
local mapType, playerContinent = ns.Maps.mapType, ns.Maps.playerContinent

-- Zone of each flight point from map data: for each map zone (C_Map.GetMapChildrenInfo)
-- the list of its points (C_TaxiMap.GetTaxiNodesForMap). Only reads map data, asks the server nothing.
-- Adds to zones: nodeID or point name -> { name = zone name, mapID = the zone's UiMapID }.
local function addZonesFromChildMaps(zones, mapID)
    if not (mapID and C_Map and C_Map.GetMapChildrenInfo and C_TaxiMap and C_TaxiMap.GetTaxiNodesForMap) then return end
    local ok, children = pcall(C_Map.GetMapChildrenInfo, mapID, mapType("Zone", ZONE_MAP_TYPE), true)
    if not (ok and type(children) == "table") then return end
    for _, child in ipairs(children) do
        local childID, zone = plain(child.mapID), plain(child.name)
        if childID and type(zone) == "string" and zone ~= "" then
            local okNodes, nodes = pcall(C_TaxiMap.GetTaxiNodesForMap, childID)
            for _, node in ipairs(okNodes and type(nodes) == "table" and nodes or {}) do
                local id, name = plain(node.nodeID), plain(node.name)
                local entry = { name = zone, mapID = childID }
                if id and zones[id] == nil then zones[id] = entry end
                if name and zones[name] == nil then zones[name] = entry end
            end
        end
    end
end

-- Zone of a flight point, where it came from and the zone's map ID (nil when unknown): a node field, the zone's
-- point list or the map at the node's position. A fourth result is the continent name when the position only gave that.
local function nodeZone(node, mapID, childZones)
    for _, field in ipairs({ "zoneName", "zone", "areaName" }) do
        local zone = plain(node[field])
        if type(zone) == "string" and zone ~= "" then return zone, "node" end
    end
    local id, name = plain(node.nodeID), plain(node.name)
    local known = (id and childZones[id]) or (name and childZones[name])
    if known then return known.name, "zoneMap", known.mapID end
    if not (mapID and C_Map and C_Map.GetMapInfoAtPosition) then return nil end
    local x, y = nodePosition(node)
    if not x then return nil end
    local ok, info = pcall(C_Map.GetMapInfoAtPosition, mapID, x, y)
    if ok and type(info) == "table" then
        local zone, infoType = plain(info.name), plain(info.mapType)
        if type(zone) ~= "string" or zone == "" or plain(info.mapID) == mapID then return nil end
        -- A map above zone level (continent, world) is no zone ("Eastern Kingdoms" must not become the zone
        -- of every point). Such a name is kept as continent instead.
        if type(infoType) == "number" and infoType < mapType("Zone", ZONE_MAP_TYPE) then
            return nil, nil, nil, (infoType == mapType("Continent", CONTINENT_MAP_TYPE)) and zone or nil
        end
        return zone, "position", plain(info.mapID)
    end
    return nil
end

-- Field names of the first raw node (diagnostics of the client's data shape).
local function fieldNames(node)
    local names = {}
    for key, value in pairs(node) do
        if type(key) == "string" then names[#names + 1] = key .. ":" .. type(value) end
    end
    table.sort(names)
    return names
end

local function addTaxiMapNodes(flights)
    if not (C_TaxiMap and C_TaxiMap.GetAllTaxiNodes and GetTaxiMapID) then return end
    local mapID = plain(GetTaxiMapID())
    local ok, taxiNodes = pcall(C_TaxiMap.GetAllTaxiNodes, mapID)
    if not (ok and type(taxiNodes) == "table") then return end
    flights.taxiMapID = mapID
    local childZones = {}
    addZonesFromChildMaps(childZones, mapID)
    local continent = playerContinent()
    if continent ~= mapID then addZonesFromChildMaps(childZones, continent) end
    local sources = {}
    for _, node in ipairs(taxiNodes) do
        if not flights.nodeFields and type(node) == "table" then flights.nodeFields = fieldNames(node) end
        local name, state = plain(node.name), plain(node.state)
        if name then
            local entry = { name = name, state = NODE_STATES[state] or state, nodeID = plain(node.nodeID) }
            local zoneOk, zone, source, zoneMapID, continentName = pcall(nodeZone, node, mapID, childZones)
            if zoneOk and zone then
                entry.zone = zone
                entry.zoneMapID = type(zoneMapID) == "number" and zoneMapID or nil
                sources[source] = (sources[source] or 0) + 1
            elseif zoneOk and type(continentName) == "string" then
                entry.continent = continentName
            end
            flights.nodes[#flights.nodes + 1] = entry
        end
    end
    if next(sources) then flights.zoneSources = sources end
end

-- Node type as a short text; empty when the client supplies something other than a plain string.
local function lowerCase(value)
    value = plain(value)
    if type(value) ~= "string" then return "" end
    return value:lower()
end

local function addLegacyTaxiNodes(nodes)
    if not (NumTaxiNodes and TaxiNodeName) then return end
    for index = 1, NumTaxiNodes() or 0 do
        local node = { name = plain(TaxiNodeName(index)) }
        if TaxiNodeGetType then node.state = lowerCase(TaxiNodeGetType(index)) end
        if TaxiNodeCost then node.costCopper = plain(TaxiNodeCost(index)) end
        if node.name then nodes[#nodes + 1] = node end
    end
end

local function collectFlightPaths()
    local flights = { npc = plain(UnitName("npc")), capturedAt = time(), nodes = {} }
    addTaxiMapNodes(flights)
    if #flights.nodes == 0 then addLegacyTaxiNodes(flights.nodes) end
    if #flights.nodes == 0 then return nil end
    return flights
end

ns.Snapshot.addWindowSection("flightPaths", collectFlightPaths, { "TAXIMAP_OPENED" })
