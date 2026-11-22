-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Sections character (who the character is and where they stand), stats (statistics from the
-- character window) and auras (active effects).

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue
local round1 = ns.round1

-- Position on the zone map in percent (as on the world map).
local function addMapPosition(c)
    if not (C_Map and C_Map.GetBestMapForUnit) then return end
    local mapID = plain(C_Map.GetBestMapForUnit("player"))
    c.mapID = mapID
    if not (mapID and C_Map.GetPlayerMapPosition) then return end
    local pos = C_Map.GetPlayerMapPosition(mapID, "player")
    if not (pos and pos.GetXY) then return end
    local rawX, rawY = pos:GetXY()
    local x, y = plain(rawX), plain(rawY)
    if x and y then
        c.x = round1(x * 100)
        c.y = round1(y * 100)
    end
end

local function addClientVersion(c)
    if not GetBuildInfo then return end
    local version, build, _, interface = GetBuildInfo()
    c.clientVersion = plain(version)
    c.clientBuild = plain(build)
    c.clientInterface = plain(interface)
end

local function collectCharacter()
    local c = {}
    c.name = plain(UnitName("player"))
    c.level = plain(UnitLevel("player"))
    local className, classToken = UnitClass("player")
    c.class = plain(className)
    c.classToken = plain(classToken)
    local raceName, raceToken = UnitRace("player")
    c.race = plain(raceName)
    c.raceToken = plain(raceToken)
    c.faction = plain(UnitFactionGroup("player"))
    c.moneyCopper = plain(GetMoney())
    c.xp = plain(UnitXP("player"))
    c.xpMax = plain(UnitXPMax("player"))
    c.restedXp = firstValue({ "GetXPExhaustion" })
    -- In an inn or city: rested XP accumulates there.
    local resting = callFirst({ "IsResting" })
    if resting then c.resting = plain(resting[1]) and true or false end
    c.zone = firstValue({ "GetRealZoneText" })
    c.subZone = firstValue({ "GetSubZoneText" })
    -- The place the Hearthstone returns to.
    c.bindLocation = firstValue({ "GetBindLocation" })
    -- Below this level quests are grey (no XP). Classically GetQuestGreenRange,
    -- in the new API UnitQuestTrivialLevelRange.
    c.questGreenRange = firstValue({ "UnitQuestTrivialLevelRange", "GetQuestGreenRange" }, "player")
    c.maxQuests = firstValue({ "C_QuestLog.GetMaxNumQuestsCanAccept", "C_QuestLog.GetMaxNumQuests" })
        or plain(MAX_QUESTLOG_QUESTS)
    -- Client language (enUS, deDE...): the server knows which text in the dump it can read as English.
    c.locale = firstValue({ "GetLocale" })
    addMapPosition(c)
    addClientVersion(c)
    c.addonVersion = ns.ADDON_VERSION
    return c
end

local STAT_NAMES = { "strength", "agility", "stamina", "intellect", "spirit" }

-- Attack power: base plus positive and negative bonus; nil when the client does not supply it.
local function attackPower(apiName)
    local results = callFirst({ apiName }, "player")
    if results and plain(results[1]) then
        return results[1] + (plain(results[2]) or 0) + (plain(results[3]) or 0)
    end
    return nil
end

local function addRangedDamage(s)
    local ranged = callFirst({ "UnitRangedDamage" }, "player")
    if ranged and plain(ranged[1]) then
        s.rangedSpeed = round1(ranged[1])
        s.rangedDamageMin = round1(plain(ranged[2]))
        s.rangedDamageMax = round1(plain(ranged[3]))
    end
end

local function addMeleeDamage(s)
    local melee = callFirst({ "UnitDamage" }, "player")
    if melee and plain(melee[1]) then
        s.meleeDamageMin = round1(melee[1])
        s.meleeDamageMax = round1(plain(melee[2]))
    end
end

local function collectStats()
    local s = {}
    s.healthMax = firstValue({ "UnitHealthMax" }, "player")
    s.manaMax = firstValue({ "UnitPowerMax" }, "player", 0)

    for index, name in ipairs(STAT_NAMES) do
        local results = callFirst({ "UnitStat" }, "player", index)
        if results then
            -- Returns: base value, value with equipment, positive bonus, negative bonus.
            s[name] = plain(results[2]) or plain(results[1])
        end
    end

    local armor = callFirst({ "UnitArmor" }, "player")
    if armor then s.armor = plain(armor[2]) or plain(armor[1]) end

    s.attackPower = attackPower("UnitAttackPower")
    s.rangedAttackPower = attackPower("UnitRangedAttackPower")

    s.critChance = round1(firstValue({ "GetCritChance" }))
    s.rangedCritChance = round1(firstValue({ "GetRangedCritChance" }))
    s.dodgeChance = round1(firstValue({ "GetDodgeChance" }))
    s.parryChance = round1(firstValue({ "GetParryChance" }))

    addRangedDamage(s)
    addMeleeDamage(s)
    s.meleeSpeed = round1(firstValue({ "UnitAttackSpeed" }, "player"))
    return s
end

local AURA_FILTERS = { "HELPFUL", "HARMFUL" }
local MAX_AURAS = 40

-- Aura end time as unix time: expirationTime is on the GetTime() clock, so
-- we convert it to the wall clock. An aura without a duration (duration 0) has no end.
local function auraTiming(aura)
    local duration, expiration = plain(aura.duration), plain(aura.expirationTime)
    if type(duration) ~= "number" or duration <= 0 then return nil, nil end
    local expiresAt
    if type(expiration) == "number" and expiration > 0 and GetTime then
        local ok, now = pcall(GetTime)
        if ok and type(plain(now)) == "number" then
            expiresAt = math.floor(time() + (expiration - now) + 0.5)
        end
    end
    return expiresAt, round1(duration)
end

local function auraEntry(aura, filter)
    local name = plain(aura.name)
    if not name then return nil end
    local expiresAt, duration = auraTiming(aura)
    return {
        name = name,
        id = plain(aura.spellId),
        icon = plain(aura.icon),
        harmful = (filter == "HARMFUL") or nil,
        stacks = (type(plain(aura.applications)) == "number" and aura.applications > 1) and aura.applications or nil,
        expiresAt = expiresAt,
        duration = duration,
    }
end

local function collectAuras()
    local auras = {}
    if not (C_UnitAuras and C_UnitAuras.GetAuraDataByIndex) then return auras end
    for _, filter in ipairs(AURA_FILTERS) do
        for index = 1, MAX_AURAS do
            local ok, aura = pcall(C_UnitAuras.GetAuraDataByIndex, "player", index, filter)
            if not ok or type(aura) ~= "table" then break end
            local entry = auraEntry(aura, filter)
            if entry then auras[#auras + 1] = entry end
        end
    end
    return auras
end

ns.Snapshot.addSection("character", collectCharacter)
ns.Snapshot.addSection("stats", collectStats)
ns.Snapshot.addSection("auras", collectAuras)
