-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Sections bags (contents of equipped bags) and equipment (equipped gear).

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue
local tooltipFor = ns.Compat.tooltipFor
local Items = ns.Items

local function collectBags()
    local bags = {}
    for bag = 0, Items.lastEquippedBag() do
        local ok, container = pcall(Items.scanContainer, bag)
        if ok and container then bags[#bags + 1] = container end
    end
    return bags
end

local AMMO_SLOT = 0

-- Every slot read goes through callFirst (pcall), so one bad slot does not spoil the rest.
local function addDurability(item, slot)
    local durability = callFirst({ "GetInventoryItemDurability" }, slot)
    if not durability then return end
    item.durability = plain(durability[1])
    item.durabilityMax = plain(durability[2])
end

local function equippedItem(slot, link)
    local item = {
        slot = Items.EQUIPMENT_SLOTS[slot] or tostring(slot),
        id = firstValue({ "GetInventoryItemID" }, "player", slot) or Items.idFromLink(link),
        name = Items.nameFromLink(link),
        quality = firstValue({ "GetInventoryItemQuality" }, "player", slot),
    }
    addDurability(item, slot)
    pcall(Items.addDetails, item, link)
    if slot == AMMO_SLOT then
        -- Ammo has no durability, but what counts is how much of it is left.
        item.count = firstValue({ "GetInventoryItemCount" }, "player", AMMO_SLOT)
        item.durability, item.durabilityMax = nil, nil
    else
        Items.attachTooltip(item, tooltipFor("GetInventoryItem", "player", slot))
    end
    return item
end

local function collectEquipment()
    local equipment = {}
    -- Slots 1-19, and last slot 0: ammo (arrows, bullets) in clients that have it.
    for i = 1, 20 do
        local slot = i % 20
        local link = firstValue({ "GetInventoryItemLink" }, "player", slot)
        if link then
            local ok, item = pcall(equippedItem, slot, link)
            if ok then equipment[#equipment + 1] = item end
        end
    end
    return equipment
end

ns.Snapshot.addSection("bags", collectBags)
ns.Snapshot.addSection("equipment", collectEquipment)
