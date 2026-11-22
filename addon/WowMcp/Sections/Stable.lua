-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section stable (from the window): pets in the stable, saved when the player opens the stable window.
-- The addon changes nothing in the stable.

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue

local CURRENT_PET_SLOT = 0 -- in the classic API slot 0 is the pet with the character
local MAX_SLOTS = 50       -- guard against an unrealistic number of slots from the API

-- Pet from a new API table (C_StableInfo).
local function petFromTable(info, slot)
    return {
        slot = plain(info.slotID) or slot,
        name = plain(info.name),
        level = plain(info.level),
        family = plain(info.familyName) or plain(info.type),
        specialization = plain(info.specialization),
    }
end

-- Pet from classic GetStablePetInfo results: icon, name, level, family, loyalty.
local function petFromValues(r, slot)
    return {
        slot = slot,
        name = plain(r[2]),
        level = plain(r[3]),
        family = plain(r[4]),
        loyalty = plain(r[5]),
    }
end

local function stabledPet(slot)
    local r = callFirst({ "C_StableInfo.GetStablePetInfo", "GetStablePetInfo" }, slot)
    if not r then return nil end
    local pet
    if type(r[1]) == "table" then
        pet = petFromTable(r[1], slot)
    else
        pet = petFromValues(r, slot)
    end
    if not pet.name then return nil end
    if slot == CURRENT_PET_SLOT then pet.current = true end
    return pet
end

-- New API: a ready list of stabled pets, preceded by the active (summoned) one with active = true.
local function petsFromList()
    local r = callFirst({ "C_StableInfo.GetStabledPetList" })
    if not r or type(r[1]) ~= "table" then return nil end
    local pets = {}
    local active = callFirst({ "C_StableInfo.GetActivePetList" })
    for i, info in ipairs(active and type(active[1]) == "table" and active[1] or {}) do
        if type(info) == "table" then
            local pet = petFromTable(info, i)
            pet.active = true
            if pet.name then pets[#pets + 1] = pet end
        end
    end
    for i, info in ipairs(r[1]) do
        if type(info) == "table" then pets[#pets + 1] = petFromTable(info, i) end
    end
    return pets
end

-- Classic API: slots from 0 (the pet with the character) to the number of purchased slots.
local function petsBySlot()
    local slots = firstValue({ "C_StableInfo.GetNumStableSlots", "GetNumStableSlots" })
    if type(slots) ~= "number" then return nil end
    local pets = {}
    for slot = CURRENT_PET_SLOT, math.min(slots, MAX_SLOTS) do
        local pet = stabledPet(slot)
        if pet then pets[#pets + 1] = pet end
    end
    return pets
end

local function collectStable()
    local pets = petsFromList() or petsBySlot()
    if not pets then return nil end
    return {
        capturedAt = time(),
        slots = firstValue({ "C_StableInfo.GetNumStableSlots", "GetNumStableSlots" }),
        pets = pets,
    }
end

ns.Snapshot.addWindowSection("stable", collectStable, { "PET_STABLE_SHOW", "PET_STABLE_UPDATE" })
