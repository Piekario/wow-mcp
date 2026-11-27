-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- /mcp item <id, item link or exact name>: reads one item from the client (the same details and tooltip lines as
-- items in the bags) and stores it in the lookups section. The client holds only items it has seen; for an
-- uncached ID it is asked to load the item, which is an item cache request like the one the item tooltip makes,
-- and the answer arrives later in GET_ITEM_INFO_RECEIVED.

local _, ns = ...
local Items, Lookups, Events = ns.Items, ns.Lookups, ns.Events
local plain, callFirst = ns.Compat.plain, ns.Compat.callFirst

local ItemLookup = {}
ns.ItemLookup = ItemLookup

local pending = {} -- itemID -> the query as typed

-- What the text names: an ID from a number or an item link, or the text itself as a name.
local function target(query)
    local id = Items.idFromLink(query) or tonumber(query:match("^%d+$"))
    return id, id or query
end

local function readItem(key, query)
    local info = callFirst({ "C_Item.GetItemInfo", "GetItemInfo" }, key)
    local name = info and plain(info[1])
    if type(name) ~= "string" then return nil end
    local link = plain(info[2])
    local id = Items.idFromLink(link) or (type(key) == "number" and key) or nil
    if not id then return nil end
    local item = { kind = "item", id = id, name = name, query = query, at = time() }
    Items.describe(item, link)
    if not item.tooltip then
        Items.attachTooltip(item, ns.Compat.tooltipFor("GetItemByID", id))
    end
    return item
end

local function store(item)
    return Lookups.add(item) and "saved" or "disabled"
end

-- Returns the status and the entry: "saved" (entry), "disabled", "pending" (the client was asked to load the item),
-- "unknown" (a name the client does not know) or "usage".
function ItemLookup.run(query)
    query = tostring(query or ""):match("^%s*(.-)%s*$")
    if query == "" then return "usage" end
    local id, key = target(query)
    local item = readItem(key, query)
    if item then return store(item), item end
    if not id then return "unknown" end
    pending[id] = query
    callFirst({ "C_Item.RequestLoadItemDataByID" }, id)
    return "pending"
end

Events.on("GET_ITEM_INFO_RECEIVED", function(itemID, success)
    local query = type(itemID) == "number" and pending[itemID]
    if not query then return end
    pending[itemID] = nil
    local item = success ~= false and readItem(itemID, query)
    if not item then
        ns.chat("The client has no item " .. itemID .. ". Check the ID.")
    elseif store(item) == "saved" then
        ns.chat(Lookups.confirmation(item))
    end
end)
