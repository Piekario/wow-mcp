-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section vendor (from the window): the vendor's offer, prices and whether they repair gear.

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue
local Items = ns.Items

-- A negative quantity means unlimited stock.
local function limitedStock(available)
    if type(available) == "number" and available >= 0 then return available end
    return nil
end

local function readMerchantItem(item, index)
    local info = callFirst({ "C_MerchantFrame.GetItemInfo" }, index)
    if info and type(info[1]) == "table" then
        local data = info[1]
        item.name = plain(data.name)
        item.priceCopper = plain(data.price)
        item.stack = plain(data.stackCount)
        item.limitedStock = limitedStock(plain(data.numAvailable))
        if data.isUsable == false then item.usable = false end
        return
    end
    local legacy = callFirst({ "GetMerchantItemInfo" }, index)
    if legacy then
        item.name = plain(legacy[1])
        item.priceCopper = plain(legacy[3])
        item.stack = plain(legacy[4])
        item.limitedStock = limitedStock(plain(legacy[5]))
    end
end

local function merchantItem(index)
    local item = { index = index }
    readMerchantItem(item, index)
    local link = firstValue({ "GetMerchantItemLink" }, index)
    if link then
        item.id = Items.idFromLink(link)
        item.name = item.name or Items.nameFromLink(link)
        pcall(Items.addDetails, item, link)
        if item.equipLoc then
            Items.attachTooltip(item, ns.Compat.tooltipFor("GetMerchantItem", index))
        end
    end
    return item
end

local function collectVendor()
    if not GetMerchantNumItems then return nil end
    local count = plain(GetMerchantNumItems())
    if type(count) ~= "number" or count <= 0 then return nil end

    local vendor = { npc = plain(UnitName("npc")), capturedAt = time(), items = {} }
    vendor.canRepair = firstValue({ "CanMerchantRepair" }) and true or false
    if vendor.canRepair then
        local cost = firstValue({ "GetRepairAllCost" })
        if type(cost) == "number" then vendor.repairCostCopper = cost end
    end

    for index = 1, count do
        local item = merchantItem(index)
        if item.name then vendor.items[#vendor.items + 1] = item end
    end
    return vendor
end

-- vendor is the last vendor, vendors: several latest (NPC name -> the same as vendor).
ns.Snapshot.addWindowSection("vendor", collectVendor, { "MERCHANT_SHOW", "MERCHANT_UPDATE" },
    { history = { name = "vendors", key = "npc", max = 10 } })
