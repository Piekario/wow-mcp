-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section equipmentSets: equipment sets saved by the player (name, items in slots,
-- whether equipped). Reads C_EquipmentSet, and in older clients the classic functions; without the API
-- the section is simply absent from the dump. Equips and changes nothing.

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue
local Items = ns.Items

local MAX_SETS = 30

-- Set items: a slot -> itemID array. Value 0 is an empty slot, 1 is an ignored slot.
local function setItems(itemIDs)
    local items = {}
    if type(itemIDs) ~= "table" then return items end
    local slots = {}
    for slot in pairs(itemIDs) do
        if type(slot) == "number" then slots[#slots + 1] = slot end
    end
    table.sort(slots)
    for _, slot in ipairs(slots) do
        local id = plain(itemIDs[slot])
        if type(id) == "number" and id > 1 then
            local name = firstValue({ "C_Item.GetItemNameByID" }, id)
            items[#items + 1] = { slot = Items.EQUIPMENT_SLOTS[slot] or tostring(slot), id = id, name = name }
        end
    end
    return items
end

local function modernSets()
    local ids = callFirst({ "C_EquipmentSet.GetEquipmentSetIDs" })
    if not (ids and type(ids[1]) == "table") then return nil end
    local sets = {}
    for _, setID in ipairs(ids[1]) do
        local info = callFirst({ "C_EquipmentSet.GetEquipmentSetInfo" }, setID)
        local name = info and plain(info[1])
        if name then
            local itemIDs = callFirst({ "C_EquipmentSet.GetItemIDs" }, setID)
            sets[#sets + 1] = {
                name = name,
                id = plain(setID),
                isEquipped = plain(info[4]) and true or false,
                missing = plain(info[8]),
                items = setItems(itemIDs and itemIDs[1]),
            }
        end
    end
    return sets
end

-- Classic API: sets by number, items by set name.
local function legacySets()
    local count = firstValue({ "GetNumEquipmentSets" })
    if type(count) ~= "number" then return nil end
    local sets = {}
    for index = 1, count do
        local info = callFirst({ "GetEquipmentSetInfo" }, index)
        local name = info and plain(info[1])
        if name then
            local itemIDs = callFirst({ "GetEquipmentSetItemIDs" }, name)
            sets[#sets + 1] = {
                name = name,
                id = plain(info[3]),
                isEquipped = plain(info[4]) and true or false,
                missing = plain(info[8]),
                items = setItems(itemIDs and itemIDs[1]),
            }
        end
    end
    return sets
end

local function collectEquipmentSets()
    local sets = modernSets() or legacySets()
    if not sets then return nil end
    while #sets > MAX_SETS do table.remove(sets) end
    return sets
end

ns.Snapshot.addSection("equipmentSets", collectEquipmentSets)
