-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Auction house price in the item tooltip: one line "AH: <price> (<age>)" when the addon
-- has a saved price for that item (auctions section). Only displays text in the tooltip,
-- no action in the game. Disabled by /mcp ahtip off.

local _, ns = ...
local plain, firstValue = ns.Compat.plain, ns.Compat.firstValue

local AuctionTooltip = {}
ns.AuctionTooltip = AuctionTooltip

local COPPER_PER_SILVER, COPPER_PER_GOLD = 100, 10000
local MINUTE, HOUR, DAY = 60, 3600, 86400
local LINE_COLOR = { 0.4, 0.8, 1 }

---------------------------------------------------------------------------
-- Line text (pure functions)
---------------------------------------------------------------------------

-- 12345 -> "1g 23s 45c"; skips zero parts (but 0 -> "0c").
function AuctionTooltip.formatMoney(copper)
    local gold = math.floor(copper / COPPER_PER_GOLD)
    local silver = math.floor(copper % COPPER_PER_GOLD / COPPER_PER_SILVER)
    local rest = copper % COPPER_PER_SILVER
    local parts = {}
    if gold > 0 then parts[#parts + 1] = gold .. "g" end
    if silver > 0 then parts[#parts + 1] = silver .. "s" end
    if rest > 0 or #parts == 0 then parts[#parts + 1] = rest .. "c" end
    return table.concat(parts, " ")
end

-- Price age in English: "just now", "15 min ago", "3 h ago", "2 days ago".
function AuctionTooltip.formatAge(seconds)
    if type(seconds) ~= "number" or seconds < MINUTE then return "just now" end
    if seconds < HOUR then return math.floor(seconds / MINUTE) .. " min ago" end
    if seconds < DAY then return math.floor(seconds / HOUR) .. " h ago" end
    local days = math.floor(seconds / DAY)
    return days .. (days == 1 and " day ago" or " days ago")
end

-- Tooltip line for a saved price, or nil when there is no price.
function AuctionTooltip.lineFor(price, now)
    if type(price) ~= "table" then return nil end
    local copper = price.minBuyout or price.minBid
    if type(copper) ~= "number" then return nil end
    return "AH: " .. AuctionTooltip.formatMoney(copper) .. " (" .. AuctionTooltip.formatAge(now - (price.seenAt or now)) .. ")"
end

---------------------------------------------------------------------------
-- Setting and tooltip
---------------------------------------------------------------------------

function AuctionTooltip.isEnabled()
    return ns.Store.settings().ahTooltip ~= false
end

function AuctionTooltip.setEnabled(enabled)
    -- On by default, so only disabling is saved.
    ns.Store.settings().ahTooltip = ns.Compat.pick(enabled, nil, false)
end

local function storedPrice(name)
    local saved = ns.Store.savedCurrentCharacter()
    local auctions = saved and saved.auctions
    if type(auctions) ~= "table" or type(auctions.prices) ~= "table" then return nil end
    return auctions.prices[name]
end

-- Item name in the tooltip: from the tooltip itself or from the ID in the tooltip data.
local function tooltipItemName(tooltip, data)
    if tooltip and type(tooltip.GetItem) == "function" then
        local ok, name = pcall(tooltip.GetItem, tooltip)
        if ok and plain(name) and name ~= "" then return name end
    end
    if type(data) == "table" and plain(data.id) then
        return firstValue({ "C_Item.GetItemNameByID", "GetItemInfo" }, data.id)
    end
    return nil
end

local function addPriceLine(tooltip, data)
    if not AuctionTooltip.isEnabled() or ns.Snapshot.isDisabled("auctions") then return end
    local name = tooltipItemName(tooltip, data)
    local line = name and AuctionTooltip.lineFor(storedPrice(name), time())
    if line then tooltip:AddLine(line, LINE_COLOR[1], LINE_COLOR[2], LINE_COLOR[3]) end
end

local function onItemTooltip(tooltip, data)
    pcall(addPriceLine, tooltip, data)
end

-- New clients: TooltipDataProcessor; older ones: the game tooltip's OnTooltipSetItem script.
function AuctionTooltip.install()
    if TooltipDataProcessor and TooltipDataProcessor.AddTooltipPostCall
        and Enum and Enum.TooltipDataType and Enum.TooltipDataType.Item ~= nil then
        return pcall(TooltipDataProcessor.AddTooltipPostCall, Enum.TooltipDataType.Item, onItemTooltip)
    end
    if GameTooltip and GameTooltip.HookScript then
        return pcall(GameTooltip.HookScript, GameTooltip, "OnTooltipSetItem", onItemTooltip)
    end
    return false
end

pcall(AuctionTooltip.install)
