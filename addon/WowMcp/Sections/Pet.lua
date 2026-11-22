-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section pet: the character's pet (happiness, loyalty, training points,
-- action bar and spellbook).

local _, ns = ...
local plain, plainFields = ns.Compat.plain, ns.Compat.plainFields
local callFirst, firstValue = ns.Compat.callFirst, ns.Compat.firstValue

-- Happiness. Forever: C_PetInfo.GetPetHappiness(); older clients: GetPetHappiness().
-- Classically: 1 = unhappy, 2 = content, 3 = happy, then % damage.
local function addHappiness(pet)
    local happiness = callFirst({ "C_PetInfo.GetPetHappiness", "GetPetHappiness" })
    if not happiness then return end
    if type(happiness[1]) == "table" then
        pet.happinessInfo = plainFields(happiness[1])
    else
        pet.happiness = plain(happiness[1])
        pet.damagePercent = plain(happiness[2])
        pet.loyaltyRate = plain(happiness[3])
    end
end

local function addLoyalty(pet)
    local loyalty = callFirst({ "C_PetInfo.GetPetLoyalty", "GetPetLoyalty" })
    if not loyalty then return end
    if type(loyalty[1]) == "table" then
        pet.loyaltyInfo = plainFields(loyalty[1])
    else
        pet.loyalty = plain(loyalty[1])
    end
end

local function addTrainingPoints(pet)
    local points = callFirst({ "C_PetInfo.GetPetTrainingPoints", "GetPetTrainingPoints" })
    if not points then return end
    if type(points[1]) == "table" then
        pet.trainingPointsInfo = plainFields(points[1])
    else
        pet.trainingPointsTotal = plain(points[1])
        pet.trainingPointsSpent = plain(points[2])
        if pet.trainingPointsTotal and pet.trainingPointsSpent then
            pet.trainingPointsFree = pet.trainingPointsTotal - pet.trainingPointsSpent
        end
    end
end

local function addExperience(pet)
    local experience = callFirst({ "C_PetInfo.GetPetExperience", "GetPetExperience" })
    if not experience then return end
    pet.xp = plain(experience[1])
    pet.xpMax = plain(experience[2])
end

local function addFoodTypes(pet)
    local foods = callFirst({ "C_PetInfo.GetPetFoodTypes", "GetPetFoodTypes" })
    if not foods then return end
    local list = {}
    for _, food in ipairs(type(foods[1]) == "table" and foods[1] or foods) do
        if type(food) == "string" then list[#list + 1] = food end
    end
    if #list > 0 then pet.foodTypes = list end
end

-- Auto-cast: nil when the ability does not support it; false when it does but is turned off.
local function autoCastState(allowed, enabled)
    if not allowed then return nil end
    return enabled and true or false
end

-- The pet's action bar: what it has available and whether it works automatically (e.g. Growl).
local function addActionBar(pet)
    if not GetPetActionInfo then return end
    pet.actionBar = {}
    for slot = 1, NUM_PET_ACTION_SLOTS or 10 do
        local ok, name, _, isToken, isActive, autoCastAllowed, autoCastEnabled, spellID = pcall(GetPetActionInfo, slot)
        if ok and plain(name) then
            if isToken and type(_G[name]) == "string" then name = _G[name] end
            pet.actionBar[#pet.actionBar + 1] = {
                slot = slot,
                name = name,
                id = plain(spellID),
                active = isActive and true or nil,
                autoCast = autoCastState(autoCastAllowed, autoCastEnabled),
            }
        end
    end
end

-- The pet's spellbook: everything it has learned, with ranks.
local function addAbilities(pet)
    if not (C_SpellBook and C_SpellBook.HasPetSpells and C_SpellBook.GetSpellBookItemInfo) then return end
    local okCount, count = pcall(C_SpellBook.HasPetSpells)
    local bank = Enum and Enum.SpellBookSpellBank and Enum.SpellBookSpellBank.Pet or 1
    if not (okCount and type(count) == "number") then return end
    pet.abilities = {}
    for i = 1, count do
        local ok, info = pcall(C_SpellBook.GetSpellBookItemInfo, i, bank)
        if ok and type(info) == "table" and info.name then
            pet.abilities[#pet.abilities + 1] = ns.Spellbook.entryFromItemInfo(info)
        end
    end
end

local function collectPet()
    local pet = { exists = UnitExists("pet") and true or false }
    if not pet.exists then
        return pet
    end

    pet.name = plain(UnitName("pet"))
    pet.level = UnitLevel("pet")
    pet.family = UnitCreatureFamily and UnitCreatureFamily("pet") or nil
    pet.dead = UnitIsDead and UnitIsDead("pet") and true or false
    pet.healthMax = firstValue({ "UnitHealthMax" }, "pet")
    addHappiness(pet)
    addLoyalty(pet)
    addTrainingPoints(pet)
    addExperience(pet)
    addFoodTypes(pet)
    addActionBar(pet)
    addAbilities(pet)
    return pet
end

ns.Snapshot.addSection("pet", collectPet)
