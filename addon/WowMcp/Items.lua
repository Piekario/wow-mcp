-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Items: hyperlink parsing, item details and container contents.
-- Shared by bags, equipment, bank, vendor, mail, auctions and Inspect.

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue
local round1 = ns.round1

local Items = {}
ns.Items = Items

-- Equipment slot names by API slot number.
Items.EQUIPMENT_SLOTS = {
    [0] = "ammo", [1] = "head", [2] = "neck", [3] = "shoulder", [4] = "shirt", [5] = "chest",
    [6] = "waist", [7] = "legs", [8] = "feet", [9] = "wrist", [10] = "hands",
    [11] = "finger1", [12] = "finger2", [13] = "trinket1", [14] = "trinket2",
    [15] = "back", [16] = "mainHand", [17] = "offHand", [18] = "ranged", [19] = "tabard",
}

-- They can be equipped, but are not gear to compare against equipment.
local NOT_GEAR = { INVTYPE_BAG = true, INVTYPE_QUIVER = true, INVTYPE_AMMO = true }

-- Item name from a hyperlink: "...|h[Name]|h..." -> "Name"
function Items.nameFromLink(link)
    if type(link) ~= "string" then return nil end
    return link:match("%[(.-)%]")
end

-- Item ID from a hyperlink: "|Hitem:12345:..." -> 12345
function Items.idFromLink(link)
    if type(link) ~= "string" then return nil end
    local id = link:match("item:(%d+)")
    return id and tonumber(id) or nil
end

-- Numeric identifiers that are the same in every client language (Enum.ItemClass, the item subclass, Enum.ItemBind):
-- the server classifies items by them, and the type/subType text is only for reading.
local function addClassIDs(item, info, link)
    item.classID = plain(info[12])
    item.subclassID = plain(info[13])
    item.bindType = plain(info[14])
    if item.classID == nil then
        -- GetItemInfoInstant: itemID, type, subType, equipLoc, icon, classID, subclassID (no server query).
        local instant = callFirst({ "C_Item.GetItemInfoInstant", "GetItemInfoInstant" }, link)
        if instant then
            item.classID = plain(instant[6])
            item.subclassID = plain(instant[7])
        end
    end
    if type(item.classID) ~= "number" then item.classID, item.subclassID = nil, nil end
    if type(item.bindType) ~= "number" then item.bindType = nil end
end

local function addItemInfo(item, link)
    local info = callFirst({ "C_Item.GetItemInfo", "GetItemInfo" }, link)
    if not (info and info[1]) then return end
    if item.quality == nil then item.quality = plain(info[3]) end
    addClassIDs(item, info, link)
    item.itemLevel = plain(info[4])
    item.requiredLevel = plain(info[5])
    item.type = plain(info[6])
    item.subType = plain(info[7])
    local equipLoc = plain(info[9])
    -- INVTYPE_NON_EQUIP_IGNORE means "cannot be equipped", i.e. the same as no field.
    if type(equipLoc) == "string" and equipLoc ~= "" and equipLoc ~= "INVTYPE_NON_EQUIP_IGNORE" then
        item.equipLoc = equipLoc
    end
    item.sellPrice = plain(info[11])
end

local function addItemStats(item, link)
    local stats = callFirst({ "C_Item.GetItemStats", "GetItemStats" }, link)
    if not (stats and type(stats[1]) == "table") then return end
    local out, tokens, any = {}, {}, false
    for key, amount in pairs(stats[1]) do
        if type(key) == "string" and type(plain(amount)) == "number" then
            -- Keys like ITEM_MOD_AGILITY_SHORT have readable names in global variables (in the client's language);
            -- statTokens keeps the keys themselves, which are the same in every language.
            local label = _G[key]
            out[type(label) == "string" and label or key] = round1(amount)
            tokens[key] = round1(amount)
            any = true
        end
    end
    if any then item.stats, item.statTokens = out, tokens end
end

function Items.addDetails(item, link)
    if type(link) ~= "string" then return end
    addItemInfo(item, link)
    addItemStats(item, link)
end

-- Weapon DPS from the stat tokens (no tooltip needed): GetItemStats gives ITEM_MOD_DAMAGE_PER_SECOND_SHORT for weapons.
local function addStatDps(item)
    local dps = item.statTokens and item.statTokens.ITEM_MOD_DAMAGE_PER_SECOND_SHORT
    if type(dps) == "number" then item.dps = round1(dps) end
end

-- What GetItemInfoInstant knows from the item ID alone (it reads the client's static tables and never queries the server).
local function addInstantInfo(item, itemID)
    local instant = callFirst({ "C_Item.GetItemInfoInstant", "GetItemInfoInstant" }, itemID)
    if not instant then return end
    item.type = plain(instant[2])
    item.subType = plain(instant[3])
    local equipLoc = plain(instant[4])
    if type(equipLoc) == "string" and equipLoc ~= "" and equipLoc ~= "INVTYPE_NON_EQUIP_IGNORE" then
        item.equipLoc = equipLoc
    end
    item.classID = plain(instant[6])
    item.subclassID = plain(instant[7])
    if type(item.classID) ~= "number" then item.classID, item.subclassID = nil, nil end
end

-- Whether the client already holds the item's data. IsItemDataCachedByID only looks in the local cache (no request);
-- when the client lacks the function we cannot tell, so we treat the data as not cached.
function Items.isDataCached(itemID)
    local result = callFirst({ "C_Item.IsItemDataCachedByID", "IsItemDataCachedByID" }, itemID)
    return result ~= nil and plain(result[1]) == true
end

-- Details of an item known only by ID (auction results), taken ONLY from what the client already holds. link names a
-- variant of the item (a random suffix) when the ID alone does not. GetItemInfo and
-- GetItemStats on an uncached item may ask the server to load it, so they run only after IsItemDataCachedByID says the data
-- is local. Without cached data only the static GetItemInfoInstant fields are filled; never a request, an event wait or a
-- tooltip. No tooltip text, so weapon DPS comes from the stat tokens and there is no speed or damage range.
-- gearOnly: stop after the static fields when the item is not gear (keeps a bounded section small).
function Items.addCachedDetails(item, itemID, gearOnly, link)
    if type(itemID) ~= "number" then return end
    local itemString = link or ("item:" .. itemID)
    pcall(addInstantInfo, item, itemID)
    if gearOnly and not Items.isGear(item) then
        item.type, item.subType, item.classID, item.subclassID = nil, nil, nil, nil
        return
    end
    if not Items.isDataCached(itemID) then return end
    pcall(addItemInfo, item, itemString)
    pcall(addItemStats, item, itemString)
    addStatDps(item)
end

-- Weapon numbers: damage range, speed and DPS as numbers, so the server does not read English tooltip text.
-- Damage and DPS lines are found with the client's own format strings (DAMAGE_TEMPLATE "%s - %s Damage",
-- DPS_TEMPLATE "(%s damage per second)"); speed is the right-hand part of the damage line (where the client puts it,
-- in every language) or, when it stands alone, the number after the client's WEAPON_SPEED label.
local NUMBER = "([%d]+[%.,]?%d*)"

-- "1.60" or "1,60" (a decimal comma) -> 1.6; the first number in the text.
local function parseNumber(text)
    local digits = type(text) == "string" and text:match("%d+[%.,]?%d*")
    if not digits then return nil end
    return tonumber((digits:gsub(",", ".")))
end

-- Lua pattern from a game format string: each %s, %d or %1$s becomes a number capture (not anchored).
local function numberPattern(format)
    local marked = format:gsub("%%%d?%$?[sd]", "\1")
    local escaped = marked:gsub("[%^%$%(%)%%%.%[%]%*%+%-%?]", "%%%0")
    return (escaped:gsub("\1", function() return NUMBER end))
end

local function findDamage(lines)
    local template = _G.DAMAGE_TEMPLATE
    if type(template) ~= "string" then return nil end
    local pattern = numberPattern(template)
    for _, line in ipairs(lines) do
        if type(line) == "string" then
            local low, high = line:match(pattern)
            low, high = parseNumber(low), parseNumber(high)
            if low and high and high > 0 then return low, high, line end
        end
    end
    return nil
end

local function findSpeed(lines, damageLine)
    local right = damageLine:match(" | (.+)$")
    local speed = parseNumber(right)
    local label = _G.WEAPON_SPEED
    if not speed and type(label) == "string" and label ~= "" then
        for _, line in ipairs(lines) do
            local at = type(line) == "string" and line:find(label, 1, true)
            if at then speed = parseNumber(line:sub(at + #label)) end
            if speed then break end
        end
    end
    if speed and speed >= 0.3 and speed <= 10 then return speed end
    return nil
end

local function findTooltipDps(lines)
    local template = _G.DPS_TEMPLATE
    if type(template) ~= "string" then return nil end
    local pattern = numberPattern(template)
    for _, line in ipairs(lines) do
        local dps = type(line) == "string" and parseNumber(line:match(pattern))
        if dps then return dps end
    end
    return nil
end

local function addWeaponNumbers(item)
    local lines = item.tooltip
    if type(lines) ~= "table" then return end
    local low, high, damageLine = findDamage(lines)
    if not low then return end
    local speed = findSpeed(lines, damageLine)
    item.damageMin, item.damageMax = low, high
    item.speed = speed and (math.floor(speed * 100 + 0.5) / 100) or nil
    local dps = item.statTokens and item.statTokens.ITEM_MOD_DAMAGE_PER_SECOND_SHORT
    dps = dps or findTooltipDps(lines) or (speed and (low + high) / 2 / speed)
    item.dps = round1(dps)
end

-- Sets the tooltip lines and, for weapons, the numbers read from them (speed, dps, damageMin, damageMax).
function Items.attachTooltip(item, lines)
    item.tooltip = lines
    if lines then pcall(addWeaponNumbers, item) end
end

-- Whether the item (after addDetails) is gear to compare against equipment.
function Items.isGear(item)
    return item.equipLoc ~= nil and not NOT_GEAR[item.equipLoc]
end

-- Details of an item known only by ID or link (e.g. a quest reward): type, level,
-- stats, and for gear the tooltip. readTooltip supplies the tooltip from a window; without it the tooltip is read by ID.
function Items.describe(item, link, readTooltip)
    item.id = item.id or Items.idFromLink(link)
    item.name = item.name or Items.nameFromLink(link)
    local itemString = link or (item.id and ("item:" .. item.id))
    pcall(Items.addDetails, item, itemString)
    if Items.isGear(item) then
        Items.attachTooltip(item, (readTooltip and readTooltip()) or (item.id and ns.Compat.tooltipFor("GetItemByID", item.id)))
    end
    return item
end

-- Number of the last equipped bag (0 is the backpack).
function Items.lastEquippedBag()
    return NUM_TOTAL_EQUIPPED_BAG_SLOTS or 5
end

-- Number of slots in a container; 0 if the container does not exist.
function Items.containerSlotCount(bag)
    local sizeResult = callFirst({ "C_Container.GetContainerNumSlots", "GetContainerNumSlots" }, bag)
    local size = sizeResult and plain(sizeResult[1]) or 0
    return type(size) == "number" and size or 0
end

-- Item in a container slot: itemID, count, quality, link, isBound (nil when the client does not say).
function Items.containerItem(bag, slot)
    if C_Container and C_Container.GetContainerItemInfo then
        local info = C_Container.GetContainerItemInfo(bag, slot)
        if type(info) == "table" then
            return plain(info.itemID), plain(info.stackCount), plain(info.quality), plain(info.hyperlink), plain(info.isBound)
        end
    elseif GetContainerItemInfo then
        local _, count, _, quality, _, _, link, _, _, itemID, isBound = GetContainerItemInfo(bag, slot)
        return plain(itemID), plain(count), plain(quality), plain(link), plain(isBound)
    end
    return nil
end

-- Quest data of a container item: isQuestItem and the ID of the quest the item begins (questID).
-- C_Container.GetContainerItemQuestInfo returns a table, the classic GetContainerItemQuestInfo several values.
local function addQuestInfo(item, bag, slot)
    local result = callFirst({ "C_Container.GetContainerItemQuestInfo", "GetContainerItemQuestInfo" }, bag, slot)
    if not result then return end
    local isQuestItem, questID
    if type(result[1]) == "table" then
        isQuestItem, questID = plain(result[1].isQuestItem), plain(result[1].questID)
    else
        isQuestItem, questID = plain(result[1]), plain(result[2])
    end
    if isQuestItem == true then item.isQuestItem = true end
    if type(questID) == "number" and questID > 0 then item.questID = questID end
end

-- Contents of one container (a bag or a bank tab).
function Items.scanContainer(bag)
    local size = Items.containerSlotCount(bag)
    if size <= 0 then return nil end

    local container = { index = bag, size = size, items = {} }
    container.name = firstValue({ "C_Container.GetBagName", "GetBagName" }, bag)
    local used = 0
    for slot = 1, size do
        local itemID, count, quality, link, isBound = Items.containerItem(bag, slot)
        if itemID or link then
            used = used + 1
            local item = {
                slot = slot,
                id = itemID or Items.idFromLink(link),
                name = Items.nameFromLink(link),
                count = count or 1,
                quality = quality,
            }
            isBound = plain(isBound)
            if type(isBound) == "boolean" then item.isBound = isBound end
            pcall(addQuestInfo, item, bag, slot)
            pcall(Items.addDetails, item, link)
            -- Full description only for equippable things, so they can be compared against equipment.
            if Items.isGear(item) then
                Items.attachTooltip(item, ns.Compat.tooltipFor("GetBagItem", bag, slot))
            end
            container.items[#container.items + 1] = item
        end
    end
    container.free = size - used
    return container
end
