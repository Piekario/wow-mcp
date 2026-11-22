-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section loot: the player's own loot since the previous dump saved to disk.
-- Reads only CHAT_MSG_LOOT messages, which the client shows in chat anyway; other
-- players' loot is skipped. After login (also after /reload, i.e. after a disk save)
-- it starts from zero, so each dump carries what came in since the previous one. The section is
-- in the dump from the start of the period (even empty), and the counters messagesSeen, messagesParsed,
-- messagesSecret and formats show whether messages arrive at all and whether they match the patterns.
-- Closed periods with loot are carried in loot.periods (oldest first, bounded; see Snapshot.closePeriod).

local _, ns = ...
local plain, callFirst = ns.Compat.plain, ns.Compat.callFirst
local Items = ns.Items

local Loot = {}
ns.Loot = Loot

Loot.MAX_ITEMS = 100

-- Patterns of own-loot messages: first the ones with a count, since a pattern without a count would swallow "x5".
local SELF_LOOT_FORMATS = {
    "LOOT_ITEM_SELF_MULTIPLE", "LOOT_ITEM_PUSHED_SELF_MULTIPLE", "LOOT_ITEM_CREATED_SELF_MULTIPLE",
    "LOOT_ITEM_BONUS_ROLL_SELF_MULTIPLE",
    "LOOT_ITEM_SELF", "LOOT_ITEM_PUSHED_SELF", "LOOT_ITEM_CREATED_SELF", "LOOT_ITEM_BONUS_ROLL_SELF",
}

-- When the client lacks these strings: the English "You receive ..." forms.
local ENGLISH_FORMATS = {
    "You receive loot: %sx%d.", "You receive item: %sx%d.", "You create: %sx%d.",
    "You receive loot: %s.", "You receive item: %s.", "You create: %s.",
}

local lootSince -- time of the dump that last reached the disk (or of login)
local formatCount = 0 -- how many game format strings were found (0 = the English fallback)

---------------------------------------------------------------------------
-- Message recognition (pure functions)
---------------------------------------------------------------------------

-- Converts a game format string ("You receive loot: %sx%d.") into a Lua pattern.
-- Returns the pattern and the field order ("link", "count"), since some languages have %1$s and %2$d.
function Loot.patternFrom(format)
    local order = {}
    local marked = format:gsub("%%(%d?)%$?([sd])", function(_, kind)
        order[#order + 1] = kind == "s" and "link" or "count"
        return kind == "s" and "\1" or "\2"
    end)
    local escaped = marked:gsub("[%^%$%(%)%%%.%[%]%*%+%-%?]", "%%%0")
    local pattern = "^" .. escaped:gsub("\1", "(.+)"):gsub("\2", "(%%d+)") .. "$"
    return pattern, order
end

local function selfLootPatterns()
    local patterns = {}
    for _, global in ipairs(SELF_LOOT_FORMATS) do
        local format = _G[global]
        if type(format) == "string" and format ~= "" then patterns[#patterns + 1] = { Loot.patternFrom(format) } end
    end
    formatCount = #patterns
    if #patterns == 0 then
        for _, format in ipairs(ENGLISH_FORMATS) do patterns[#patterns + 1] = { Loot.patternFrom(format) } end
    end
    return patterns
end

-- Item link and count from the player's own loot message; nil for other players' loot.
function Loot.parseOwn(message, patterns)
    if type(message) ~= "string" then return nil end
    for _, p in ipairs(patterns or selfLootPatterns()) do
        local captures = { message:match(p[1]) }
        if #captures > 0 then
            local fields = {}
            for i, field in ipairs(p[2]) do fields[field] = captures[i] end
            if fields.link and fields.link:find("|Hitem:") then
                return fields.link, tonumber(fields.count) or 1
            end
        end
    end
    return nil
end

---------------------------------------------------------------------------
-- Recording in the dump
---------------------------------------------------------------------------

local function newSection(since)
    return { since = since, items = {}, messagesSeen = 0, messagesParsed = 0, messagesSecret = 0 }
end

local function lootSection()
    local current = ns.Snapshot.current()
    if type(current.loot) ~= "table" then current.loot = newSection(lootSince or time()) end
    local loot = current.loot
    if type(loot.items) ~= "table" then loot.items = {} end
    return loot
end

-- Diagnostic counter (older dumps may not have it).
local function bump(section, field)
    section[field] = (type(section[field]) == "number" and section[field] or 0) + 1
end

local function findItem(items, id, name)
    for _, item in ipairs(items) do
        if (id and item.id == id) or (not id and item.name == name) then return item end
    end
    return nil
end

-- The longest-ago looted fall off when there are too many different items.
local function trimOldest(items)
    while #items > Loot.MAX_ITEMS do
        local oldest = 1
        for i, item in ipairs(items) do
            if (item.lastAt or 0) < (items[oldest].lastAt or 0) then oldest = i end
        end
        table.remove(items, oldest)
    end
end

local function itemQuality(link)
    local info = callFirst({ "C_Item.GetItemInfo", "GetItemInfo" }, link)
    return info and plain(info[3]) or nil
end

function Loot.record(link, count)
    local items = lootSection().items
    local id, name = Items.idFromLink(link), Items.nameFromLink(link)
    if not (id or name) then return end
    local item = findItem(items, id, name)
    if not item then
        item = { id = id, name = name, quality = itemQuality(link), count = 0 }
        items[#items + 1] = item
    end
    item.count = item.count + count
    item.lastAt = time()
    trimOldest(items)
end

local function onLootMessage(message)
    local section = lootSection()
    bump(section, "messagesSeen")
    local text = plain(message)
    if message ~= nil and text == nil then
        bump(section, "messagesSecret") -- the client hid the content ("secret"), it cannot be read
        return
    end
    local link, amount = Loot.parseOwn(text)
    if link then
        bump(section, "messagesParsed")
        Loot.record(link, amount)
    end
    section.formats = formatCount
end

-- A closed period is worth carrying only when it has own loot.
function Loot.hasContent(section)
    return type(section.items) == "table" and #section.items > 0
end

-- After login (and after /reload) the previous dump is already on disk together with its loot; the closed period
-- is carried in loot.periods (Snapshot.startPeriod), so a second save before the server reads the file loses nothing.
local function startLootPeriod()
    lootSince = ns.Snapshot.startPeriod("loot", newSection, Loot.hasContent)
end

ns.Snapshot.addWindowSection("loot", nil, nil, { quiet = true })
ns.Events.on("PLAYER_LOGIN", function() pcall(startLootPeriod) end)
ns.Events.on("CHAT_MSG_LOOT", ns.Snapshot.whenEnabled("loot", function(message) pcall(onLootMessage, message) end))
