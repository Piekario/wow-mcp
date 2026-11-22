-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section guild (from the window): the player's guild and the member list the client already has in memory.
-- Collected on GUILD_ROSTER_UPDATE (the client sends it e.g. after the guild window opens).
-- The addon never asks the server for the list (no GuildRoster or C_GuildInfo.GuildRoster).
-- For a large guild (over 200 people) it saves only those who are online.

local _, ns = ...
local plain, callFirst = ns.Compat.plain, ns.Compat.callFirst

local MAX_MEMBERS = 500
local ONLINE_ONLY_ABOVE = 200
local MIN_SECONDS_BETWEEN = 5 -- the event is frequent; the same state is not read again

local lastRead = 0
local lastSignature

local function textOrNil(value)
    value = plain(value)
    if type(value) == "string" and value ~= "" then return value end
    return nil
end

-- GetGuildRosterInfo: fullName, rank, rankIndex, level, class, zone, note, officerNote, online, status, classFileName, ...
local function readMember(index)
    local info = callFirst({ "GetGuildRosterInfo", "C_GuildInfo.GetGuildRosterInfo" }, index)
    if not info then return nil end
    local name = textOrNil(info[1])
    if not name then return nil end
    return {
        name = name, rank = textOrNil(info[2]), rankIndex = plain(info[3]), level = plain(info[4]),
        class = textOrNil(info[5]), zone = textOrNil(info[6]), online = plain(info[9]) and true or false,
    }
end

local function readMembers(total)
    local onlineOnly = total > ONLINE_ONLY_ABOVE
    local members = {}
    for index = 1, total do
        local member = readMember(index)
        if member and (member.online or not onlineOnly) then
            members[#members + 1] = member
            if #members >= MAX_MEMBERS then break end
        end
    end
    return members, onlineOnly
end

local function signature(guild)
    local parts = { tostring(guild.name), tostring(guild.rank), tostring(guild.total), tostring(guild.online) }
    for _, m in ipairs(guild.members or {}) do
        parts[#parts + 1] = (m.name or "") .. "|" .. (m.zone or "") .. "|" .. tostring(m.level) .. "|" .. tostring(m.online)
    end
    return table.concat(parts, "\n")
end

-- A character without a guild: only the fact remains instead of a list (inGuild = false), so that having no guild
-- can be told apart from a list the client has not shown yet.
local function notInGuild()
    return { inGuild = false, capturedAt = time() }
end

local function forgetGuild()
    ns.Snapshot.current().guild = notInGuild()
    lastSignature = nil
end

local function collectGuild()
    local now = time()
    if now - lastRead < MIN_SECONDS_BETWEEN and type(ns.Snapshot.current().guild) == "table" then return nil end
    lastRead = now

    local inGuild = callFirst({ "IsInGuild" })
    if inGuild and not plain(inGuild[1]) then
        forgetGuild()
        return nil
    end
    local mine = callFirst({ "GetGuildInfo" }, "player")
    local guild = { inGuild = true, capturedAt = now }
    if mine then
        guild.name, guild.rank, guild.rankIndex = textOrNil(mine[1]), textOrNil(mine[2]), plain(mine[3])
    end

    -- GetNumGuildMembers: total, onlineAndMobile, online. Returns 0 without a loaded list.
    local counts = callFirst({ "GetNumGuildMembers", "C_GuildInfo.GetNumGuildMembers" })
    local total = counts and plain(counts[1])
    if type(total) == "number" and total > 0 then
        guild.total = total
        guild.online = plain(counts[3]) or plain(counts[2])
        guild.members, guild.onlineOnly = readMembers(total)
        guild.onlineOnly = guild.onlineOnly or nil
    else
        -- List not loaded yet: keep the previous member data, refresh only the player's guild.
        local previous = ns.Snapshot.current().guild
        if type(previous) == "table" then
            guild.total, guild.online, guild.members, guild.onlineOnly = previous.total, previous.online, previous.members, previous.onlineOnly
        end
    end
    if not (guild.name or guild.members) then return nil end

    local current = signature(guild)
    if current == lastSignature and type(ns.Snapshot.current().guild) == "table" then return nil end
    lastSignature = current
    return guild
end

-- On every dump: only IsInGuild and GetGuildInfo("player") (client memory, no GuildRoster).
-- Without a guild: { inGuild = false }. In a guild: the previous record with the list, or just the name and rank.
local function refreshMembership(previous)
    local inGuild = callFirst({ "IsInGuild" })
    if not inGuild then return previous end -- client without this function: what was there stays
    if not plain(inGuild[1]) then
        if type(previous) == "table" and previous.inGuild == false then return previous end
        return notInGuild()
    end
    if type(previous) == "table" and previous.inGuild ~= false then
        previous.inGuild = true
        return previous
    end
    local mine = callFirst({ "GetGuildInfo" }, "player")
    local guild = { inGuild = true, capturedAt = time() }
    if mine then
        guild.name, guild.rank, guild.rankIndex = textOrNil(mine[1]), textOrNil(mine[2]), plain(mine[3])
    end
    return guild
end

-- GUILD_ROSTER_UPDATE: the list; PLAYER_GUILD_UPDATE: the player's name and rank (e.g. right after login).
ns.Snapshot.addWindowSection("guild", collectGuild, { "GUILD_ROSTER_UPDATE", "PLAYER_GUILD_UPDATE" }, { quiet = true })
ns.Snapshot.refreshOnDump("guild", refreshMembership)
