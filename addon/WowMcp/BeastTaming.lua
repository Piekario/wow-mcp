-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Taming observations (Hunter): when the player starts Tame Beast, the target is remembered; when a pet from that
-- creature appears shortly after, its spellbook abilities are saved with the beast (Beasts.record, source "tamed").
-- Abilities are read only at that moment: later the pet also has ranks its Hunter taught it, which the wild beast
-- does not have. The addon only listens; it casts nothing.

local _, ns = ...
local plain = ns.Compat.plain
local Beasts = ns.Beasts

local TAME_BEAST = 1515    -- spell ID, the same in every client language
local WINDOW_SECONDS = 60  -- the channel lasts 20 s; the pet and its spellbook arrive a moment after

local pending -- { info = Beasts.unitInfo("target"), at = time() } while a taming may still finish

-- UNIT_SPELLCAST_START / UNIT_SPELLCAST_CHANNEL_START(unit, castGUID, spellID)
local function onCastStart(unit, _, spellID)
    if plain(unit) ~= "player" or plain(spellID) ~= TAME_BEAST or not Beasts.isHunter() then return end
    local info = Beasts.unitInfo("target")
    if info then pending = { info = info, at = time() } end
end

-- Whether the pet is the beast being tamed: by the creature ID in the pet's GUID, otherwise by family.
local function isTamedBeast(info)
    local petNpcId = Beasts.petNpcId()
    if petNpcId then return petNpcId == info.npcId end
    local family = ns.Compat.firstValue({ "UnitCreatureFamily" }, "pet")
    return family ~= nil and family == info.family
end

-- Spellbook abilities with their rank text, without stances and commands.
local function tamedAbilities()
    local abilities = {}
    for _, entry in ipairs(ns.Pet.spellbook() or {}) do
        if not entry.petAction and type(plain(entry.name)) == "string" then
            abilities[#abilities + 1] = { name = entry.name, rank = plain(entry.rank), id = plain(entry.id) }
        end
    end
    return abilities
end

-- Records the tamed beast; keeps waiting (inside the window) until the pet's spellbook has abilities.
local function onPetChanged()
    if not pending then return end
    if time() - pending.at > WINDOW_SECONDS then
        pending = nil
        return
    end
    if not (ns.Compat.firstValue({ "UnitExists" }, "pet") and isTamedBeast(pending.info)) then return end
    local abilities = tamedAbilities()
    Beasts.record(pending.info, "tamed", { abilities = abilities })
    if #abilities > 0 then pending = nil end
end

local function guarded(handler)
    return ns.Snapshot.whenEnabled("beasts", function(...) pcall(handler, ...) end)
end

ns.Events.on("UNIT_SPELLCAST_START", guarded(onCastStart))
ns.Events.on("UNIT_SPELLCAST_CHANNEL_START", guarded(onCastStart))
ns.Events.on("UNIT_PET", guarded(onPetChanged))
ns.Events.on("PET_BAR_UPDATE", guarded(onPetChanged))
ns.Events.on("SPELLS_CHANGED", guarded(onPetChanged))
