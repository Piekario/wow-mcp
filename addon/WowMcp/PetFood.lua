-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Pet food in the bags: the client itself says which bag items the pet can eat (C_PetInfo.CanPetEatItem), so the
-- addon needs no diet table. It only asks; it never feeds the pet. Forever has the function (probe of addon 0.7.0), but
-- its argument is not confirmed: the item ID is tried first, then the item link, and the form that returned a boolean
-- is saved as checkedWith.

local _, ns = ...
local plain, callFirst = ns.Compat.plain, ns.Compat.callFirst
local Items = ns.Items

local PetFood = {}
ns.PetFood = PetFood

PetFood.MAX_ITEMS = 40
PetFood.PROBE_ITEMS = 3

local CONSUMABLE, FOOD_AND_DRINK, TRADE_GOODS = 0, 5, 7 -- Enum.ItemClass / Enum.ItemConsumableSubclass
local FORMS = { "itemID", "itemLink" }

local function canEatFunction()
    return ns.Compat.resolve("C_PetInfo.CanPetEatItem")
end

-- classID and subclassID: GetItemInfoInstant reads static data; GetItemInfo answers from the cache for bag items.
local function itemClass(itemID, link)
    local instant = callFirst({ "C_Item.GetItemInfoInstant", "GetItemInfoInstant" }, itemID or link)
    local classID, subclassID = instant and plain(instant[6]), instant and plain(instant[7])
    if type(classID) ~= "number" then
        local info = callFirst({ "C_Item.GetItemInfo", "GetItemInfo" }, link or itemID)
        classID, subclassID = info and plain(info[12]), info and plain(info[13])
    end
    return classID, subclassID
end

-- Raw meat and fish are Trade Goods, cooked food is Food & Drink.
local function isConsumableFood(classID, subclassID)
    return classID == CONSUMABLE and subclassID == FOOD_AND_DRINK
end

-- Distinct bag items in bag order: { id, name, link, count }; count is the total over all stacks.
local function bagItems()
    local list, byID = {}, {}
    for bag = 0, Items.lastEquippedBag() do
        for slot = 1, Items.containerSlotCount(bag) do
            local ok, itemID, count, _, link = pcall(Items.containerItem, bag, slot)
            itemID = ok and (itemID or Items.idFromLink(link)) or nil
            if type(itemID) == "number" then
                local entry = byID[itemID]
                if not entry then
                    entry = { id = itemID, name = Items.nameFromLink(link), link = link, count = 0 }
                    byID[itemID] = entry
                    list[#list + 1] = entry
                end
                entry.count = entry.count + (type(count) == "number" and count or 1)
            end
        end
    end
    return list
end

local function argumentFor(item, form)
    if form == "itemID" then return item.id end
    return item.link
end

-- A boolean answer of CanPetEatItem for one argument form, or nil when it raised an error or returned something else.
local function ask(fn, item, form)
    local argument = argumentFor(item, form)
    if argument == nil then return nil end
    local ok, result = pcall(fn, argument)
    if not ok then return nil end
    result = plain(result)
    if type(result) == "boolean" then return result end
    return nil
end

-- Tries the form that already worked first, then the other one.
local function canEat(fn, item, state)
    local order = FORMS
    if state.form == "itemLink" then order = { "itemLink", "itemID" } end
    for _, form in ipairs(order) do
        local answer = ask(fn, item, form)
        if answer ~= nil then
            state.form = state.form or form
            return answer
        end
    end
    return nil
end

-- The pet's verdict on every food candidate in the bags; nil when the client has no CanPetEatItem or it never
-- answered with a boolean. Trade Goods the pet refuses are left out (cloth and leather are not food).
function PetFood.check()
    local fn = canEatFunction()
    if not fn then return nil end
    local state, items, asked = {}, {}, 0
    for _, item in ipairs(bagItems()) do
        local classID, subclassID = itemClass(item.id, item.link)
        local consumable = isConsumableFood(classID, subclassID)
        if (consumable or classID == TRADE_GOODS) and #items < PetFood.MAX_ITEMS then
            asked = asked + 1
            local answer = canEat(fn, item, state)
            if answer == true or (answer == false and consumable) then
                items[#items + 1] = { id = item.id, name = item.name, count = item.count, canEat = answer }
            end
        end
    end
    if asked > 0 and not state.form then return nil end
    return { checkedWith = state.form, items = items }
end

-- One return value as the probe saves it: its type and, when plain, the value.
local function describeValue(value)
    return { type = type(value), value = plain(value) }
end

local function pack(...)
    return { n = select("#", ...), ... }
end

local function probeCall(fn, item, form)
    local argument = argumentFor(item, form)
    local call = { itemID = item.id, form = form }
    if argument == nil then
        call.skipped = "no " .. form
        return call
    end
    local results = pack(pcall(fn, argument))
    call.ok = results[1]
    if not call.ok then
        call.error = tostring(results[2]):sub(1, 200)
        return call
    end
    call.count = results.n - 1
    call.results = {}
    for i = 1, math.min(call.count, 4) do call.results[i] = describeValue(results[i + 1]) end
    return call
end

-- For /mcp probe: the raw shape of what CanPetEatItem returns for the first few bag items with each argument form,
-- so one run in the client confirms its signature.
function PetFood.probe()
    local fn = canEatFunction()
    local out = { exists = fn ~= nil, calls = {} }
    if not fn then return out end
    for i, item in ipairs(bagItems()) do
        if i > PetFood.PROBE_ITEMS then break end
        for _, form in ipairs(FORMS) do out.calls[#out.calls + 1] = probeCall(fn, item, form) end
    end
    return out
end
