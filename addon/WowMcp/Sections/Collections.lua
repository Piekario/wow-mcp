-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section collections: the mounts and companion pets the character knows. Mounts come from C_MountJournal when the
-- client has it, otherwise from the companion API (GetNumCompanions / GetCompanionInfo "MOUNT"); companions from the
-- companion API ("CRITTER"), otherwise from C_PetJournal (whose list follows the journal's own filters).
-- Absent when the client has none of these. Nothing is summoned, dismissed or marked as a favorite.

local _, ns = ...
local Compat = ns.Compat
local plain, callFirst, firstValue = Compat.plain, Compat.callFirst, Compat.firstValue

local MAX_PER_KIND = 300

local function newKind(source)
    return { source = source, list = {}, notListed = 0 }
end

local function add(kind, entry)
    if not entry.name then return end
    if #kind.list < MAX_PER_KIND then
        kind.list[#kind.list + 1] = entry
    else
        kind.notListed = kind.notListed + 1
    end
end

local function finish(kind)
    if kind and kind.notListed == 0 then kind.notListed = nil end
    return kind
end

local function journalMounts()
    local ids = callFirst({ "C_MountJournal.GetMountIDs" })
    ids = ids and ids[1]
    if type(ids) ~= "table" then return nil end
    local mounts = newKind("C_MountJournal")
    for _, mountID in ipairs(ids) do
        local info = callFirst({ "C_MountJournal.GetMountInfoByID" }, mountID)
        -- Returns: name, spellID, icon, isActive, isUsable, sourceType, isFavorite, isFactionSpecific, faction,
        -- shouldHideOnChar, isCollected, mountID.
        if info and plain(info[11]) then
            add(mounts, { name = plain(info[1]), spellID = plain(info[2]), id = plain(info[12]) or plain(mountID) })
        end
    end
    return mounts
end

-- GetCompanionInfo returns: creatureID, name, spellID, icon, isSummoned.
local function companionList(companionType)
    local count = tonumber(firstValue({ "GetNumCompanions" }, companionType))
    if not count or not Compat.resolve("GetCompanionInfo") then return nil end
    local kind = newKind("GetCompanionInfo")
    for index = 1, count do
        local info = callFirst({ "GetCompanionInfo" }, companionType, index)
        if info then add(kind, { name = plain(info[2]), id = plain(info[1]), spellID = plain(info[3]) }) end
    end
    return kind
end

-- C_PetJournal.GetPetInfoByIndex returns: petID, speciesID, owned, customName, level, favorite, isRevoked,
-- speciesName, icon, petType, companionID.
local function journalPets()
    local count = tonumber(firstValue({ "C_PetJournal.GetNumPets" }))
    if not count or not Compat.resolve("C_PetJournal.GetPetInfoByIndex") then return nil end
    local pets = newKind("C_PetJournal")
    for index = 1, count do
        local info = callFirst({ "C_PetJournal.GetPetInfoByIndex" }, index)
        if info and plain(info[3]) then
            local customName = plain(info[4])
            add(pets, {
                name = Compat.pick(customName ~= nil and customName ~= "", customName, plain(info[8])),
                species = plain(info[8]), id = plain(info[2]), level = plain(info[5]),
            })
        end
    end
    return pets
end

local function collectCollections()
    local collections = {
        mounts = finish(journalMounts() or companionList("MOUNT")),
        companions = finish(companionList("CRITTER") or journalPets()),
    }
    if not (collections.mounts or collections.companions) then return nil end
    return collections
end

ns.Snapshot.addSection("collections", collectCollections)
