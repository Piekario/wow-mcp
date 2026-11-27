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

-- The friends API gives the localized class name only; the token (PALADIN) is what the server maps to a group role.
local function classTokenOf(className)
    if not className then return nil end
    for _, names in ipairs({ LOCALIZED_CLASS_NAMES_MALE or {}, LOCALIZED_CLASS_NAMES_FEMALE or {} }) do
        for token, localized in pairs(names) do
            if localized == className then return token end
        end
    end
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

-- Battle.net friends who are in this game right now, as their character. The account name and BattleTag are not read
-- into the dump. C_BattleNet.GetFriendAccountInfo: { gameAccountInfo = { characterName, className, characterLevel,
-- areaName, isOnline, clientProgram } }.
local WOW_CLIENT = BNET_CLIENT_WOW or "WoW"

local function battleNetFriend(index)
    local info = callFirst({ "C_BattleNet.GetFriendAccountInfo" }, index)
    local game = info and type(info[1]) == "table" and info[1].gameAccountInfo
    if type(game) ~= "table" or plain(game.clientProgram) ~= WOW_CLIENT or not plain(game.isOnline) then return nil end
    return {
        name = textOrNil(game.characterName), level = plain(game.characterLevel), class = textOrNil(game.className),
        area = textOrNil(game.areaName), connected = true, battleNet = true,
    }
end

local function addBattleNetFriends(friends)
    local count = firstValue({ "BNGetNumFriends" })
    if type(count) ~= "number" then return end
    local known = {}
    for _, friend in ipairs(friends) do known[friend.name] = true end
    for index = 1, math.min(count, MAX_FRIENDS) do
        local friend = battleNetFriend(index)
        if friend and friend.name and not known[friend.name] and #friends < MAX_FRIENDS then
            friend.classToken = classTokenOf(friend.class)
            friends[#friends + 1] = friend
        end
    end
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
        if friend and friend.name then
            friend.classToken = classTokenOf(friend.class)
            friends[#friends + 1] = friend
        end
    end
    addBattleNetFriends(friends)
    -- The event arrives often (status changes): without changes in the list we do not save again.
    local current = signature(friends)
    if current == lastSignature and type(ns.Snapshot.current().friends) == "table" then return nil end
    lastSignature = current

    local online = 0
    for _, f in ipairs(friends) do
        if f.connected then online = online + 1 end
    end
    return { capturedAt = time(), total = math.max(count, #friends), online = online, friends = friends }
end

-- Forever build 71037 (real client, 2026-11-24): opening the friends list fired no FRIENDLIST_UPDATE, so the section
-- was never saved. Every dump now reads the list the client already holds; it still never asks the server for it.
-- An empty list read this way is not saved: before the client has received the list it also reports no friends.
local function refreshFriends(previous)
    local collected = collectFriends()
    if collected and (#collected.friends > 0 or type(previous) == "table") then return collected end
    return previous
end

ns.Snapshot.addWindowSection("friends", collectFriends, { "FRIENDLIST_UPDATE" }, { quiet = true })
ns.Snapshot.refreshOnDump("friends", refreshFriends)
