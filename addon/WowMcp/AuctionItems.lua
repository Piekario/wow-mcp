-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Auction items: the name and details of what an auction row sells. A browse row is an item key (item ID, item level and
-- random suffix: "of the Eagle" is a suffix, and the base item alone has none of its stats), a listing has its own link.
-- Only gear gets details, without localized stat labels, because the auctions section is bounded.

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue
local Items = ns.Items

local AuctionItems = {}
ns.AuctionItems = AuctionItems

local function suffixOf(key)
    local suffix = plain(key.itemSuffix)
    if type(suffix) == "number" and suffix ~= 0 then return suffix end
    return nil
end

-- The suffix is the seventh field of an item string: item:id:enchant:gem1:gem2:gem3:gem4:suffix.
local function keyLink(key, itemID)
    local sample = firstValue({ "C_AuctionHouse.GetItemKeySampleLink" }, key)
    if type(sample) == "string" and sample:find("item:", 1, true) then return sample end
    local suffix = suffixOf(key)
    if suffix then return ("item:%d::::::%d"):format(itemID, suffix) end
    return "item:" .. itemID
end

local function keyTooltip(key, itemID)
    return ns.Compat.tooltipFor("GetItemKey", itemID, plain(key.itemLevel) or 0, suffixOf(key) or 0)
end

-- The key info names the base item on some clients; the tooltip title is what the auction row shows.
local function keyName(key, itemID, tooltip)
    local info = callFirst({ "C_AuctionHouse.GetItemKeyInfo" }, key)
    local name = info and type(info[1]) == "table" and plain(info[1].itemName) or nil
    if type(name) ~= "string" or name == "" then
        name = firstValue({ "C_Item.GetItemNameByID" }, itemID)
    end
    local title = tooltip and tooltip[1]
    if type(name) == "string" and type(title) == "string" and title ~= name and title:find(name, 1, true) then
        return title
    end
    return name
end

local function withoutLabels(details)
    details.stats = nil
    return details
end

-- Name and details of a browse row, from what the client already holds. complete is false while the client is still
-- loading the item: the caller asks again when the data arrives.
function AuctionItems.describeKey(key)
    local itemID = type(key) == "table" and plain(key.itemID)
    if type(itemID) ~= "number" then return nil, {}, true end
    local details = { suffix = suffixOf(key) }
    Items.addCachedDetails(details, itemID, true, keyLink(key, itemID))
    local gear, cached = Items.isGear(details), Items.isDataCached(itemID)
    local tooltip = cached and keyTooltip(key, itemID) or nil
    if gear then Items.attachTooltip(details, tooltip) end
    local name = keyName(key, itemID, tooltip)
    return name, withoutLabels(details), name ~= nil and (cached or not gear)
end

-- Name and details of one listing: its link carries the suffix and the client already shows the item.
function AuctionItems.describeLink(link)
    local details = {}
    pcall(Items.addDetails, details, link)
    if not Items.isGear(details) then return Items.nameFromLink(link), {} end
    Items.attachTooltip(details, ns.Compat.tooltipFor("GetHyperlink", link))
    return Items.nameFromLink(link), withoutLabels(details)
end

-- Asks the client to load the data of an item the player's search returned (what the auction window does for the
-- rows it draws). It is an item cache request, not an auction house query.
function AuctionItems.requestData(key)
    local itemID = type(key) == "table" and plain(key.itemID)
    if type(itemID) == "number" then callFirst({ "C_Item.RequestLoadItemDataByID" }, itemID) end
end
