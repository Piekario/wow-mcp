-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section group: party or raid composition at dump time. Solo = nil.

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue

-- C_PartyInfo.GetLootMethod (Forever) returns an Enum.LootMethod number; the classic GetLootMethod gives these strings.
local LOOT_METHODS = { [0] = "freeforall", [1] = "roundrobin", [2] = "master", [3] = "group", [4] = "needbeforegreed", [5] = "personal" }

local Group = {}
ns.Group = Group

-- Name, level, class and race of a unit; nil when there is no unit or the name is a secret.
-- Shared by group members and players from the Inspect window.
function Group.unitIdentity(unit)
    local name = plain(UnitName(unit))
    if not name then return nil end
    local className, classToken = UnitClass(unit)
    return {
        name = name, level = plain(UnitLevel(unit)), class = plain(className), classToken = plain(classToken),
        race = plain((UnitRace(unit))),
    }
end

local function memberFromUnit(unit)
    if not UnitExists or not UnitExists(unit) then return nil end
    local m = Group.unitIdentity(unit)
    if not m then return nil end
    m.role = firstValue({ "UnitGroupRolesAssigned" }, unit)
    if m.role == "NONE" then m.role = nil end
    m.leader = firstValue({ "UnitIsGroupLeader", "UnitIsPartyLeader" }, unit) and true or nil
    m.online = firstValue({ "UnitIsConnected" }, unit)
    m.dead = firstValue({ "UnitIsDeadOrGhost" }, unit) and true or nil
    m.healthMax = firstValue({ "UnitHealthMax" }, unit)
    m.powerMax = firstValue({ "UnitPowerMax" }, unit)
    local power = callFirst({ "UnitPowerType" }, unit)
    if power then m.powerType = plain(power[2]) end
    if unit ~= "player" then
        -- In a group the Forever client returns both values (in range, range checked) as secrets: test only plain ones.
        local inRange = callFirst({ "UnitInRange" }, unit)
        local checked = inRange and plain(inRange[2])
        if checked then
            local value = plain(inRange[1])
            if value ~= nil then m.inRange = value and true or false end
        end
    end
    return m
end

local function raidMembers()
    local members = {}
    for i = 1, firstValue({ "GetNumGroupMembers", "GetNumRaidMembers" }) or 0 do
        local m = memberFromUnit("raid" .. i)
        if m then
            local r = callFirst({ "GetRaidRosterInfo" }, i)
            if r then
                -- name, rank, subgroup, level, class, fileName, zone, online, isDead, role, isML
                m.raidRank = plain(r[2])
                m.subgroup = plain(r[3])
                m.zone = plain(r[7])
                m.masterLooter = r[11] and true or nil
            end
            members[#members + 1] = m
        end
    end
    return members
end

-- The player and the other group members (party1..N does not include the player).
local function partyMembers(count)
    local members = {}
    local me = memberFromUnit("player")
    if me then members[#members + 1] = me end
    for i = 1, count do
        local m = memberFromUnit("party" .. i)
        if m then members[#members + 1] = m end
    end
    return members
end

local function collectGroup()
    local inRaid = firstValue({ "IsInRaid" }) or (firstValue({ "GetNumRaidMembers" }) or 0) > 0
    local group = {}
    if inRaid then
        group.type = "raid"
        group.members = raidMembers()
    else
        local n = firstValue({ "GetNumSubgroupMembers", "GetNumPartyMembers" }) or 0
        if n == 0 then return nil end
        group.type = "party"
        group.members = partyMembers(n)
    end
    if #group.members == 0 then return nil end
    group.size = #group.members
    local loot = callFirst({ "C_PartyInfo.GetLootMethod", "GetLootMethod" })
    if loot then
        local method = plain(loot[1])
        group.lootMethod = LOOT_METHODS[method] or method
    end
    group.lootThreshold = firstValue({ "GetLootThreshold" })
    return group
end

ns.Snapshot.addSection("group", collectGroup)
