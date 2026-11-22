-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section auctions (from the window): prices from search results that the player opened
-- in the auction window, and the player's own auctions. The addon searches and posts nothing;
-- it only reads what the client has already shown. Prices accumulate between visits
-- (the latest observation wins), and seen keeps several last minimums to judge the trend. Since 0.13.0 a gear entry also has
-- the item details the client already holds (Items.addCachedDetails: never a request), so the server can compare it.

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue
local Items, pick = ns.Items, ns.Compat.pick

local AUCTION_MAX_ITEMS = 400
local AUCTION_MAX_SEEN = 5
local SAME_VISIT_SECONDS = 600
local AUCTION_STATUS_SOLD = 1

---------------------------------------------------------------------------
-- Storing prices in the dump
---------------------------------------------------------------------------

local function auctionsSection()
    local current = ns.Snapshot.current()
    if type(current.auctions) ~= "table" then current.auctions = {} end
    local auctions = current.auctions
    if type(auctions.prices) ~= "table" then auctions.prices = {} end
    return auctions
end

-- Removes the longest-unseen prices when there are too many.
local function pruneAuctionPrices(prices)
    local names = {}
    for name in pairs(prices) do names[#names + 1] = name end
    if #names <= AUCTION_MAX_ITEMS then return end
    table.sort(names, function(a, b) return (prices[a].seenAt or 0) > (prices[b].seenAt or 0) end)
    for i = AUCTION_MAX_ITEMS + 1, #names do prices[names[i]] = nil end
end

-- Appends a minimum to the history; the same visit overwrites the last point instead of adding a new one.
local function recordSeenMinimum(entry, minBuyout, now)
    entry.seen = type(entry.seen) == "table" and entry.seen or {}
    local last = entry.seen[#entry.seen]
    if last and now - (last.at or 0) < SAME_VISIT_SECONDS then
        last.at, last.minBuyout = now, minBuyout
    else
        entry.seen[#entry.seen + 1] = { at = now, minBuyout = minBuyout }
        while #entry.seen > AUCTION_MAX_SEEN do table.remove(entry.seen, 1) end
    end
end

-- Item details that stay on a price entry (0.13.0). Filled from the client's cache only (Items.addCachedDetails); a later
-- visit with uncached data keeps what an earlier one saved. The section is bounded (400 prices), so there is no tooltip text,
-- no localized stat labels (statTokens say the same) and details only for gear: details for every price would add about
-- 165 kB at the cap of 400, gear only about 135 kB (a cached weapon entry grows by about 340 bytes).
local function addItemDetails(entry, itemID)
    local details = {}
    Items.addCachedDetails(details, itemID, true)
    details.stats = nil
    for key, value in pairs(details) do entry[key] = value end
end

-- Stores the results gathered from one screen: observations[name] = {id, unitBuyout, unitBid, quantity, listings}.
local function storeAuctionPrices(observations, source)
    local auctions = auctionsSection()
    local now = time()
    auctions.capturedAt = now
    for name, o in pairs(observations) do
        local entry = auctions.prices[name] or { name = name }
        entry.id = o.id or entry.id
        pcall(addItemDetails, entry, entry.id)
        entry.minBuyout = o.unitBuyout
        entry.minBid = o.unitBid
        entry.quantity = o.quantity
        entry.listings = o.listings
        entry.seenAt = now
        entry.source = source
        if o.unitBuyout then recordSeenMinimum(entry, o.unitBuyout, now) end
        auctions.prices[name] = entry
    end
    pruneAuctionPrices(auctions.prices)
    ns.Snapshot.windowCaptured("auctions")
end

local function unitPrice(total, count)
    return math.floor(total / count + 0.5)
end

-- Adds one auction to an item's observation: totals of items and auctions, minimum per-unit prices.
local function observe(observations, name, id, count, buyout, bid)
    if type(name) ~= "string" or name == "" then return end
    count = (type(count) == "number" and count > 0) and count or 1
    local o = observations[name] or { id = id, quantity = 0, listings = 0 }
    o.id = o.id or id
    o.quantity = o.quantity + count
    o.listings = o.listings + 1
    if type(buyout) == "number" and buyout > 0 then
        local unit = unitPrice(buyout, count)
        if not o.unitBuyout or unit < o.unitBuyout then o.unitBuyout = unit end
    end
    if type(bid) == "number" and bid > 0 then
        local unit = unitPrice(bid, count)
        if not o.unitBid or unit < o.unitBid then o.unitBid = unit end
    end
    observations[name] = o
end

-- Item name by ID for the new API, which provides only the ID.
local function itemNameByID(itemID)
    if not itemID then return nil end
    return firstValue({ "C_Item.GetItemNameByID", "GetItemInfo" }, itemID)
end

---------------------------------------------------------------------------
-- Classic auction window (GetAuctionItemInfo)
---------------------------------------------------------------------------

-- Row of the classic auction list. Returns name, count, minBid, buyout, owner, saleStatus, itemID.
local function classicAuctionRow(kind, index)
    local r = callFirst({ "GetAuctionItemInfo" }, kind, index)
    if not r or not plain(r[1]) then return nil end
    -- Newer clients have a levelColHeader string after the level (r[6]) and later fields shifted by one.
    local o = (type(r[7]) == "string" or r[7] == nil and type(r[8]) == "number") and 1 or 0
    local minBid, buyout = plain(r[7 + o]), plain(r[9 + o])
    local owner = plain(pick(o == 1, r[14], r[12]))
    local saleStatus = pick(o == 1, plain(r[16]), nil)
    local itemID = pick(o == 1, plain(r[17]), nil)
    local link = firstValue({ "GetAuctionItemLink" }, kind, index)
    return r[1], plain(r[3]), minBid, buyout, owner, saleStatus, itemID or Items.idFromLink(link)
end

-- Current page of search results.
local function captureClassicAuctionList()
    local counts = callFirst({ "GetNumAuctionItems" }, "list")
    local batch = counts and plain(counts[1])
    if type(batch) ~= "number" or batch <= 0 then return end
    local observations = {}
    for index = 1, batch do
        local name, count, minBid, buyout, _, _, itemID = classicAuctionRow("list", index)
        observe(observations, name, itemID, count, buyout, minBid)
    end
    storeAuctionPrices(observations, "list")
    local total = plain(counts[2])
    if type(total) == "number" and total > batch then
        auctionsSection().lastSearchPartial = { shown = batch, total = total }
    else
        auctionsSection().lastSearchPartial = nil
    end
end

local function classicOwnedAuctions()
    local counts = callFirst({ "GetNumAuctionItems" }, "owner")
    local batch = counts and plain(counts[1])
    if type(batch) ~= "number" then return nil end
    local owned = {}
    for index = 1, batch do
        local name, count, minBid, buyout, _, saleStatus, itemID = classicAuctionRow("owner", index)
        if name then
            owned[#owned + 1] = {
                name = name, id = itemID, count = count, buyout = buyout, bid = minBid,
                sold = (saleStatus == AUCTION_STATUS_SOLD) or nil,
            }
        end
    end
    return owned
end

---------------------------------------------------------------------------
-- New auction window (C_AuctionHouse)
---------------------------------------------------------------------------

-- Adds a browse result to the observation. Several item keys (e.g. different levels)
-- may share a name: we take the lowest per-unit price and sum the quantities, as in observe.
local function observeBrowseResult(observations, name, id, unitBuyout, quantity)
    local o = observations[name]
    if not o then
        observations[name] = { id = id, unitBuyout = unitBuyout, quantity = quantity, listings = nil }
        return
    end
    o.id = o.id or id
    if unitBuyout and (not o.unitBuyout or unitBuyout < o.unitBuyout) then o.unitBuyout = unitBuyout end
    if quantity then o.quantity = (o.quantity or 0) + quantity end
end

-- Browse results (minimum per-unit price for each item).
local function captureBrowseResults()
    local r = callFirst({ "C_AuctionHouse.GetBrowseResults" })
    if not r or type(r[1]) ~= "table" then return end
    local observations = {}
    for _, result in ipairs(r[1]) do
        local key = type(result.itemKey) == "table" and result.itemKey or {}
        local id = plain(key.itemID)
        local name = itemNameByID(id)
        if name then
            observeBrowseResult(observations, name, id, plain(result.minPrice), plain(result.totalQuantity))
        end
    end
    storeAuctionPrices(observations, "browse")
end

-- Commodity details after clicking a result.
local function captureCommodityResults(itemID)
    local n = firstValue({ "C_AuctionHouse.GetNumCommoditySearchResults" }, itemID)
    if type(n) ~= "number" or n <= 0 then return end
    local name = itemNameByID(itemID)
    if not name then return end
    local observations = {}
    for index = 1, n do
        local r = callFirst({ "C_AuctionHouse.GetCommoditySearchResultInfo" }, itemID, index)
        local info = r and r[1]
        if type(info) == "table" then
            local qty = plain(info.quantity) or 1
            observe(observations, name, itemID, qty, (plain(info.unitPrice) or 0) * qty, nil)
        end
    end
    storeAuctionPrices(observations, "commodity")
end

-- Item (non-commodity) details after clicking a result.
local function captureItemResults(itemKey)
    if type(itemKey) ~= "table" then return end
    local n = firstValue({ "C_AuctionHouse.GetNumItemSearchResults" }, itemKey)
    if type(n) ~= "number" or n <= 0 then return end
    local id = plain(itemKey.itemID)
    local name = itemNameByID(id)
    if not name then return end
    local observations = {}
    for index = 1, n do
        local r = callFirst({ "C_AuctionHouse.GetItemSearchResultInfo" }, itemKey, index)
        local info = r and r[1]
        if type(info) == "table" then
            observe(observations, name, id, plain(info.quantity) or 1, plain(info.buyoutAmount), plain(info.bidAmount))
        end
    end
    storeAuctionPrices(observations, "item")
end

local function modernOwnedAuctions(count)
    local owned = {}
    for index = 1, count do
        local r = callFirst({ "C_AuctionHouse.GetOwnedAuctionInfo" }, index)
        local info = r and r[1]
        if type(info) == "table" then
            local key = type(info.itemKey) == "table" and info.itemKey or {}
            local id = plain(key.itemID)
            owned[#owned + 1] = {
                name = Items.nameFromLink(plain(info.itemLink)) or itemNameByID(id), id = id,
                count = plain(info.quantity), buyout = plain(info.buyoutAmount), bid = plain(info.bidAmount),
                sold = (info.status == AUCTION_STATUS_SOLD) or nil, timeLeftSeconds = plain(info.timeLeftSeconds),
            }
        end
    end
    return owned
end

---------------------------------------------------------------------------
-- Auction window events
---------------------------------------------------------------------------

-- The player's own auctions (the Auctions tab in the auction window), from the new or classic API.
local function captureOwnedAuctions()
    local owned
    local n = firstValue({ "C_AuctionHouse.GetNumOwnedAuctions" })
    if type(n) == "number" then
        owned = modernOwnedAuctions(n)
    else
        owned = classicOwnedAuctions()
        if not owned then return end
    end
    local auctions = auctionsSection()
    auctions.owned = owned
    auctions.ownedAt = time()
    ns.Snapshot.windowCaptured("auctions")
end

local function captureAuctionVisit()
    local auctions = auctionsSection()
    auctions.capturedAt = time()
    auctions.lastVisitAt = time()
    auctions.npc = plain(UnitName("npc"))
    ns.Snapshot.windowCaptured("auctions")
end

local function onAuctionHouseShow()
    pcall(captureAuctionVisit)
    pcall(captureClassicAuctionList)
end

-- Event handler: does nothing when the section is disabled; a read error does not escape.
local function guarded(capture)
    return ns.Snapshot.whenEnabled("auctions", function(...) pcall(capture, ...) end)
end

-- Events: the classic window (older clients) and the new C_AuctionHouse. Forever 1.60.1 (per /mcp probe)
-- knows only the new ones: AUCTION_HOUSE_BROWSE_RESULTS_*, ITEM_SEARCH_RESULTS_UPDATED,
-- COMMODITY_SEARCH_RESULTS_UPDATED and OWNED_AUCTIONS_UPDATED; the client harmlessly rejects unknown events.
ns.Snapshot.addWindowSection("auctions")
ns.Events.on("AUCTION_HOUSE_SHOW", ns.Snapshot.whenEnabled("auctions", onAuctionHouseShow))
ns.Events.on("AUCTION_ITEM_LIST_UPDATE", guarded(captureClassicAuctionList))
ns.Events.on("AUCTION_OWNED_LIST_UPDATE", guarded(captureOwnedAuctions))
ns.Events.on("AUCTION_HOUSE_BROWSE_RESULTS_UPDATED", guarded(captureBrowseResults))
ns.Events.on("AUCTION_HOUSE_BROWSE_RESULTS_ADDED", guarded(captureBrowseResults))
ns.Events.on("COMMODITY_SEARCH_RESULTS_UPDATED", guarded(captureCommodityResults))
ns.Events.on("ITEM_SEARCH_RESULTS_UPDATED", guarded(captureItemResults))
ns.Events.on("OWNED_AUCTIONS_UPDATED", guarded(captureOwnedAuctions))
