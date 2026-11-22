-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section buyback (from the window): the vendor's buyback list (recently sold items).
-- Read-only from the open vendor window; buys nothing back.

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue
local Items = ns.Items

local function buybackItem(index)
    local item = { index = index }
    -- GetBuybackItemInfo: name, icon, price, quantity, numAvailable, isUsable (price for the whole stack).
    local info = callFirst({ "GetBuybackItemInfo", "C_MerchantFrame.GetBuybackItemInfo" }, index)
    if info then
        if type(info[1]) == "table" then
            local data = info[1]
            item.name, item.priceCopper, item.count = plain(data.name), plain(data.price), plain(data.quantity)
        else
            item.name, item.priceCopper, item.count = plain(info[1]), plain(info[3]), plain(info[4])
        end
    end
    local link = firstValue({ "GetBuybackItemLink" }, index)
    if link then
        item.id = Items.idFromLink(link)
        item.name = item.name or Items.nameFromLink(link)
    end
    item.id = item.id or firstValue({ "C_MerchantFrame.GetBuybackItemID" }, index)
    return item
end

local function collectBuyback()
    local count = firstValue({ "GetNumBuybackItems", "C_MerchantFrame.GetNumBuybackItems" })
    if type(count) ~= "number" then return nil end
    -- The buyback list makes sense only with the vendor window open; otherwise the old record stays.
    local merchantItems = firstValue({ "GetMerchantNumItems" })
    if count == 0 and not (type(merchantItems) == "number" and merchantItems > 0) then return nil end

    local buyback = { npc = plain(UnitName("npc")), capturedAt = time(), items = {} }
    for index = 1, count do
        local item = buybackItem(index)
        if item.name then buyback.items[#buyback.items + 1] = item end
    end
    return buyback
end

ns.Snapshot.addWindowSection("buyback", collectBuyback, { "MERCHANT_SHOW", "MERCHANT_UPDATE" })
