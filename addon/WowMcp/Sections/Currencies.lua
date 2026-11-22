-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section currencies: currencies and tokens (the Currency window). The client may have C_CurrencyInfo
-- or the old GetCurrencyListInfo; collapsed headers hide their currencies, so they are skipped.

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue

-- Currency list row: name, isHeader, quantity, maxQuantity.
local function currencyRow(results)
    if type(results[1]) == "table" then
        local info = results[1]
        return plain(info.name), info.isHeader, plain(info.quantity), plain(info.maxQuantity)
    end
    -- Old version: name, isHeader, isExpanded, isUnused, isWatched, count, ...
    return plain(results[1]), results[2], plain(results[6]), nil
end

local function currencyEntry(index, name, quantity, maxQuantity, header)
    local entry = { name = name, quantity = quantity, group = header }
    if type(maxQuantity) == "number" and maxQuantity > 0 then entry.max = maxQuantity end
    local link = firstValue({ "C_CurrencyInfo.GetCurrencyListLink", "GetCurrencyListLink" }, index)
    if type(link) == "string" then entry.id = tonumber(link:match("currency:(%d+)")) end
    return entry
end

local function collectCurrencies()
    local sizeResult = callFirst({ "C_CurrencyInfo.GetCurrencyListSize", "GetCurrencyListSize" })
    local size = sizeResult and plain(sizeResult[1])
    if type(size) ~= "number" then return nil end
    local currencies, header = {}, nil
    for index = 1, size do
        local results = callFirst({ "C_CurrencyInfo.GetCurrencyListInfo", "GetCurrencyListInfo" }, index)
        if results then
            local name, isHeader, quantity, maxQuantity = currencyRow(results)
            if name and isHeader then
                header = name
            elseif name then
                currencies[#currencies + 1] = currencyEntry(index, name, quantity, maxQuantity, header)
            end
        end
    end
    return currencies
end

ns.Snapshot.addSection("currencies", collectCurrencies)
