-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Encounter Journal reader: instances, bosses and loot through the EJ_* functions and C_EncounterJournal.
-- Selecting an instance and resetting the loot filters changes what the journal window shows, so a read
-- remembers the window's selection and filters and puts them back afterwards. Nothing here is gameplay.
-- The EJ_* names are the ones mainline clients use; the Forever client was only probed for C_EncounterJournal.

local _, ns = ...
local plain, callFirst, resolve = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.resolve

local EJ = {}
ns.EncounterJournal = EJ

local MAX_INSTANCES = 150
local MAX_TIERS = 12
local MAX_ENCOUNTERS = 20

local REQUIRED = { "EJ_GetInstanceByIndex", "EJ_SelectInstance", "EJ_GetEncounterInfoByIndex", "EJ_GetNumLoot" }
local LOOT_INFO = { "C_EncounterJournal.GetLootInfoByIndex", "EJ_GetLootInfoByIndex" }

local function numberOrNil(value)
    value = plain(value)
    if type(value) == "number" then return value end
    return nil
end

-- The first journal function this client lacks, or nil when everything needed is there.
function EJ.missingApi()
    for _, path in ipairs(REQUIRED) do
        if not resolve(path) then return path end
    end
    if not (resolve(LOOT_INFO[1]) or resolve(LOOT_INFO[2])) then return LOOT_INFO[1] end
    return nil
end

-- Instances of one tier (or of the current one): { id, name, raid, mapID, tier }. EJ_GetInstanceByIndex returns
-- instanceID, name, description, bgImage, buttonImage1, loreImage, buttonImage2, dungeonAreaMapID, link,
-- shouldDisplayDifficulty, mapID (11: the instance's map ID, as in GetInstanceInfo and the group finder).
local function scanTier(list, seen, tier)
    for _, isRaid in ipairs({ false, true }) do
        for index = 1, MAX_INSTANCES do
            local r = callFirst({ "EJ_GetInstanceByIndex" }, index, isRaid)
            local id = r and numberOrNil(r[1])
            local name = r and plain(r[2])
            if not (id and type(name) == "string") then break end
            if not seen[id] then
                seen[id] = true
                list[#list + 1] = { id = id, name = name, raid = isRaid or nil, mapID = numberOrNil(r[11]), tier = tier }
            end
        end
    end
end

local rememberState, restoreSelection

-- Instances of every journal tier (the journal lists one tier at a time), and how many tiers there are (nil when the
-- client has no tiers). The tier, instance and boss the window had selected are put back afterwards.
function EJ.instances()
    local list, seen = {}, {}
    local tiers = numberOrNil(ns.Compat.firstValue({ "EJ_GetNumTiers" }))
    if not (tiers and tiers > 0 and resolve("EJ_SelectTier")) then
        scanTier(list, seen, nil)
        return list, nil
    end
    local state = rememberState()
    pcall(function()
        for tier = 1, math.min(tiers, MAX_TIERS) do
            if callFirst({ "EJ_SelectTier" }, tier) then scanTier(list, seen, tier) end
        end
    end)
    restoreSelection(state)
    return list, tiers
end

local function isInInstance()
    local inside = callFirst({ "IsInInstance" })
    return inside ~= nil and plain(inside[1]) == true
end
EJ.isInInstance = isInInstance

-- True when the journal's addon is loadable but not loaded yet, so the journal has no data to give.
function EJ.notLoadedYet()
    local loaded = callFirst({ "C_AddOns.IsAddOnLoaded", "IsAddOnLoaded" }, "Blizzard_EncounterJournal")
    if not loaded or plain(loaded[1]) ~= false then return false end
    local loadable = callFirst({ "C_AddOns.IsAddOnLoadable", "IsAddOnLoadable" }, "Blizzard_EncounterJournal")
    return not loadable or plain(loadable[1]) ~= false
end

-- Loads the game's own journal module, as opening the journal window does; the EJ_* functions list nothing before it.
-- Returns false and the client's reason when the client refuses: WoW: Forever answers WRONG_GAME_TYPE although
-- IsAddOnLoadable says the module is loadable.
function EJ.loadJournal()
    local loaded = callFirst({ "C_AddOns.LoadAddOn", "LoadAddOn" }, "Blizzard_EncounterJournal")
    if loaded and plain(loaded[1]) ~= false and not EJ.notLoadedYet() then return true end
    local reason = loaded and plain(loaded[2])
    return false, type(reason) == "string" and reason ~= "" and reason or "unknown"
end

-- The journal instance of the dungeon the character is in, or nil outside an instance.
function EJ.currentInstanceID()
    if not isInInstance() then return nil end
    local uiMapID = ns.Compat.firstValue({ "C_Map.GetBestMapForUnit" }, "player")
    if type(uiMapID) ~= "number" then return nil end
    return numberOrNil(ns.Compat.firstValue({ "C_EncounterJournal.GetInstanceForGameMap", "EJ_GetInstanceForMap" }, uiMapID))
end

-- The journal window's selection and loot filters, to put back after a read.
function rememberState()
    local state = {}
    state.tier = numberOrNil(ns.Compat.firstValue({ "EJ_GetCurrentTier" }))
    local frame = _G.EncounterJournal
    if type(frame) == "table" then
        state.instanceID, state.encounterID = numberOrNil(frame.instanceID), numberOrNil(frame.encounterID)
    end
    local filter = callFirst({ "EJ_GetLootFilter" })
    if filter then state.classID, state.specID = numberOrNil(filter[1]), numberOrNil(filter[2]) end
    state.slotFilter = numberOrNil(ns.Compat.firstValue({ "C_EncounterJournal.GetSlotFilter", "EJ_GetSlotFilter" }))
    return state
end

-- Shows the loot of every class and slot, so the read does not depend on filters the player set. A filter the
-- client does not let us read is left alone, because it could not be put back.
local function clearFilters(state)
    if state.classID then callFirst({ "EJ_ResetLootFilter" }) end
    if state.slotFilter then callFirst({ "C_EncounterJournal.ResetSlotFilter", "EJ_ResetSlotFilter" }) end
end

-- The tier, instance and boss the window had selected (a tier change does not keep the instance in every client).
function restoreSelection(state)
    local restored = true
    local function call(paths, ...)
        if not callFirst(paths, ...) then restored = false end
    end
    if state.tier then call({ "EJ_SelectTier" }, state.tier) end
    if state.instanceID then call({ "EJ_SelectInstance" }, state.instanceID) end
    if state.encounterID then call({ "EJ_SelectEncounter" }, state.encounterID) end
    return restored
end

-- Returns true when everything the read changed could be put back.
local function restoreState(state)
    local restored = true
    if state.classID and not callFirst({ "EJ_SetLootFilter" }, state.classID, state.specID or 0) then restored = false end
    if state.slotFilter and not callFirst({ "C_EncounterJournal.SetSlotFilter", "EJ_SetSlotFilter" }, state.slotFilter) then
        restored = false
    end
    return restoreSelection(state) and restored
end

-- Bosses: { id, name }. EJ_GetEncounterInfoByIndex returns name, description, journalEncounterID, ...
local function readEncounters(instanceID)
    local list = {}
    for index = 1, MAX_ENCOUNTERS do
        local r = callFirst({ "EJ_GetEncounterInfoByIndex" }, index, instanceID)
        local name = r and plain(r[1])
        if type(name) ~= "string" then break end
        list[#list + 1] = { id = numberOrNil(r[3]), name = name }
    end
    return list
end

-- One loot row. C_EncounterJournal.GetLootInfoByIndex returns a table (itemID, encounterID, name, itemQuality,
-- slot, armorType, link); the older EJ_GetLootInfoByIndex returns itemID, encounterID, name, icon, slot,
-- armorType, link. A row whose item data is not loaded yet has an ID but no name.
local function lootRow(index)
    local r = callFirst(LOOT_INFO, index)
    if not r then return nil end
    local info = r[1]
    if type(info) == "table" then
        return { id = numberOrNil(info.itemID), encounterID = numberOrNil(info.encounterID), name = plain(info.name),
                 quality = numberOrNil(info.itemQuality), slotText = plain(info.slot),
                 armorType = plain(info.armorType), link = plain(info.link) }
    end
    return { id = numberOrNil(r[1]), encounterID = numberOrNil(r[2]), name = plain(r[3]), slotText = plain(r[5]),
             armorType = plain(r[6]), link = plain(r[7]) }
end

local function appendLoot(rows, seen, maxItems)
    local count = numberOrNil(ns.Compat.firstValue({ "EJ_GetNumLoot" })) or 0
    for index = 1, count do
        if #rows >= maxItems then return true end
        local row = lootRow(index)
        if row and row.id and not seen[row.id] then
            seen[row.id] = true
            rows[#rows + 1] = row
        end
    end
    return false
end

-- The instance's loot. With only the instance selected the journal lists the loot of all its bosses; when a
-- client lists nothing that way, each boss is selected in turn.
local function readLoot(encounters, maxItems)
    local rows, seen = {}, {}
    local truncated = appendLoot(rows, seen, maxItems)
    if #rows == 0 then
        for _, encounter in ipairs(encounters) do
            if encounter.id and callFirst({ "EJ_SelectEncounter" }, encounter.id) then
                local before = #rows
                truncated = appendLoot(rows, seen, maxItems) or truncated
                for i = before + 1, #rows do rows[i].encounterID = rows[i].encounterID or encounter.id end
            end
        end
    end
    return rows, truncated
end

-- Reads one instance: { encounters, rows, truncated, restored }, or nil and an error text.
function EJ.read(instance, maxItems)
    local state = rememberState()
    local ok, result = pcall(function()
        clearFilters(state)
        if instance.tier then callFirst({ "EJ_SelectTier" }, instance.tier) end
        if not callFirst({ "EJ_SelectInstance" }, instance.id) then error("EJ_SelectInstance failed") end
        local encounters = readEncounters(instance.id)
        local rows, truncated = readLoot(encounters, maxItems)
        return { encounters = encounters, rows = rows, truncated = truncated or nil }
    end)
    local restored = restoreState(state)
    if not ok then return nil, tostring(result) end
    result.restored = restored
    return result
end
