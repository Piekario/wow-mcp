-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section friends (from the window): the friends list the client already has in memory.
-- Collected on FRIENDLIST_UPDATE (the client sends it e.g. after the social window opens).
-- The addon never asks the server for the list (no ShowFriends or C_FriendList.ShowFriends).

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue

local MAX_FRIENDS = 100

local lastSignature

local function textOrNil(value)
    value = plain(value)
    if type(value) == "string" and value ~= "" then return value end
    return nil
end

-- C_FriendList.GetFriendInfoByIndex: table { name, level, className, area, connected, notes, ... }.
local function modernFriend(index)
    local info = callFirst({ "C_FriendList.GetFriendInfoByIndex" }, index)
    local data = info and info[1]
    if type(data) ~= "table" then return nil end
    return {
        name = textOrNil(data.name), level = plain(data.level), class = textOrNil(data.className),
        area = textOrNil(data.area), connected = plain(data.connected) and true or false, notes = textOrNil(data.notes),
    }
end

-- GetFriendInfo: name, level, class, area, connected, status, note.
local function legacyFriend(index)
    local info = callFirst({ "GetFriendInfo" }, index)
    if not info then return nil end
    return {
        name = textOrNil(info[1]), level = plain(info[2]), class = textOrNil(info[3]), area = textOrNil(info[4]),
        connected = plain(info[5]) and true or false, notes = textOrNil(info[7]),
    }
end

local function signature(friends)
    local parts = {}
    for _, f in ipairs(friends) do
        parts[#parts + 1] = table.concat({ f.name or "", tostring(f.level), f.area or "", tostring(f.connected), f.notes or "" }, "|")
    end
    return table.concat(parts, "\n")
end

local function collectFriends()
    local count = firstValue({ "C_FriendList.GetNumFriends", "GetNumFriends" })
    if type(count) ~= "number" then return nil end
    local friends = {}
    for index = 1, math.min(count, MAX_FRIENDS) do
        local friend = modernFriend(index) or legacyFriend(index)
        if friend and friend.name then friends[#friends + 1] = friend end
    end
    -- The event arrives often (status changes): without changes in the list we do not save again.
    local current = signature(friends)
    if current == lastSignature and type(ns.Snapshot.current().friends) == "table" then return nil end
    lastSignature = current

    local online = 0
    for _, f in ipairs(friends) do
        if f.connected then online = online + 1 end
    end
    return { capturedAt = time(), total = count, online = online, friends = friends }
end

ns.Snapshot.addWindowSection("friends", collectFriends, { "FRIENDLIST_UPDATE" }, { quiet = true })
