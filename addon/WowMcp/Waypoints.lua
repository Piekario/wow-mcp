-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Waypoints: coordinates in WowMcp chat lines as clickable links that set a TomTom waypoint (and its arrow).
--
-- TomTom is optional and never required: without it (or without the client's link handler API) a place stays
-- plain text, and /mcp quest suggests TomTom once per session. A waypoint is set only when the player clicks
-- a link; nothing here runs on its own. TomTom's API (v4.4.1 on Forever): TomTom:AddWaypoint(uiMapID, x, y, opts)
-- with x and y from 0 to 1, the same call Questie makes; the UI map ID comes from the quest database (QuestDb),
-- or from the zone name through the client's map data (Maps).

local _, ns = ...
local QuestDb, Maps, chat = ns.QuestDb, ns.Maps, ns.chat

local Waypoints = {}
ns.Waypoints = Waypoints

local LINK_TYPE = "wowmcp"
local LINK_COLOR = "|cff71d5ff"
local SOURCE = "WowMcp"
local DEFAULT_TITLE = "WowMcp waypoint"
local MAX_TITLES = 200

-- Titles of the links printed this session, by "mapID:x:y" (a title can hold any text, a link field cannot).
local titles, titleCount = {}, 0
local zoneIDs
local linksEnabled = false
local hintShown = false

local function tomTom()
    local addon = _G.TomTom
    if type(addon) == "table" and type(addon.AddWaypoint) == "function" then return addon end
    return nil
end

function Waypoints.available()
    return linksEnabled and tomTom() ~= nil
end

local function mapIDByZoneName(zone)
    if type(zone) ~= "string" then return nil end
    if not (zoneIDs and next(zoneIDs)) then zoneIDs = Maps.zoneIDsByName() end
    return zoneIDs[zone]
end

-- { mapID, x, y } for a location from the quest database ({ zone, areaID, x, y }, coordinates 0-100), or nil
-- when it has no coordinates (inside an instance) or no known map.
function Waypoints.resolve(location)
    if type(location) ~= "table" or location.inInstance then return nil end
    local x, y = location.x, location.y
    if type(x) ~= "number" or type(y) ~= "number" then return nil end
    local mapID = QuestDb.uiMapID(location.areaID) or mapIDByZoneName(location.zone)
    if not mapID then return nil end
    return { mapID = mapID, x = x, y = y }
end

local function key(mapID, x, y)
    return mapID .. ":" .. x .. ":" .. y
end

local function rememberTitle(place, title)
    if type(title) ~= "string" or title == "" then return end
    if titleCount >= MAX_TITLES then titles, titleCount = {}, 0 end
    local k = key(place.mapID, place.x, place.y)
    if titles[k] == nil then titleCount = titleCount + 1 end
    titles[k] = title
end

-- The text of a place for chat: a link that sets a TomTom waypoint titled `title` when TomTom and the client's
-- link API are there and the map is known; otherwise the text unchanged.
function Waypoints.chatText(location, title, text)
    if not Waypoints.available() then return text end
    local place = Waypoints.resolve(location)
    if not place then return text end
    rememberTitle(place, title)
    return ("%s|H%s:way:%s|h[%s]|h|r"):format(LINK_COLOR, LINK_TYPE, key(place.mapID, place.x, place.y), text)
end

-- After a lookup that printed coordinates: once per session, says that TomTom would make them clickable.
function Waypoints.suggestTomTom()
    if hintShown or not linksEnabled or tomTom() then return end
    hintShown = true
    chat("Tip: install TomTom and the coordinates in /mcp quest become links that set a waypoint with one click.")
end

-- Sets the waypoint of a clicked link. Returns true when the link was ours.
function Waypoints.click(link)
    if type(link) ~= "string" then return false end
    local mapID, x, y = link:match(LINK_TYPE .. ":way:(%d+):([%d%.]+):([%d%.]+)")
    mapID, x, y = tonumber(mapID), tonumber(x), tonumber(y)
    if not (mapID and x and y) then return false end
    local addon = tomTom()
    if not addon then
        chat("TomTom is not loaded, so the waypoint cannot be set.")
        return true
    end
    local title = titles[key(mapID, x, y)] or DEFAULT_TITLE
    local ok = pcall(addon.AddWaypoint, addon, mapID, x / 100, y / 100, { title = title, from = SOURCE, crazy = true })
    if not ok then chat("TomTom did not accept the waypoint (map " .. mapID .. ").") end
    return true
end

local function response(name, default)
    local responses = _G.LinkProcessorResponse
    return (type(responses) == "table" and responses[name] ~= nil) and responses[name] or default
end

-- The client's link handler API (LinkUtil.RegisterLinkHandler, which Questie also uses on Forever): a handler,
-- not a replaced function, so no Blizzard code is tainted. A modified click (shift-click) keeps its default.
local function registerLinkHandler()
    local linkUtil = _G.LinkUtil
    if type(linkUtil) ~= "table" or type(linkUtil.RegisterLinkHandler) ~= "function" then return false end
    local ok = pcall(linkUtil.RegisterLinkHandler, LINK_TYPE, function(link)
        local modified = _G.IsModifiedClick
        if type(modified) == "function" and modified() then return response("Unhandled", false) end
        if Waypoints.click(link) then return response("Handled", true) end
        return response("Unhandled", false)
    end)
    return ok
end

linksEnabled = registerLinkHandler()
