-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section mail (from the window): the mailbox. Sender, subject, gold, cash on
-- delivery (COD), attachments and when the mail expires.

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue
local round1, Items, pick = ns.round1, ns.Items, ns.Compat.pick

local SECONDS_PER_DAY = 86400

local function positiveAmount(amount)
    if type(plain(amount)) == "number" and amount > 0 then return amount end
    return nil
end

local function attachment(index, slot)
    local r = callFirst({ "GetInboxItem" }, index, slot)
    if not (r and plain(r[1])) then return nil end
    -- Newer clients: name, itemID, texture, count, quality, canUse; older: name, texture, count, quality, canUse.
    local newer = type(r[6]) == "boolean"
    local item = {
        name = r[1],
        count = plain(pick(newer, r[4], r[3])) or 1,
        quality = plain(pick(newer, r[5], r[4])),
    }
    local link = firstValue({ "GetInboxItemLink" }, index, slot)
    item.id = Items.idFromLink(link) or pick(newer, plain(r[2]), nil)
    return item
end

local function attachments(index)
    local items = {}
    for slot = 1, ATTACHMENTS_MAX_RECEIVE or 12 do
        items[#items + 1] = attachment(index, slot)
    end
    return items
end

local function inboxMessage(index)
    local h = callFirst({ "GetInboxHeaderInfo" }, index)
    if not h then return nil end
    -- packageIcon, stationeryIcon, sender, subject, money, CODAmount, daysLeft, hasItem, wasRead, wasReturned
    local days = plain(h[7])
    local message = {
        sender = plain(h[3]), subject = plain(h[4]),
        moneyCopper = positiveAmount(h[5]),
        codCopper = positiveAmount(h[6]),
        daysLeft = type(days) == "number" and round1(days) or nil,
        expiresAt = type(days) == "number" and (time() + math.floor(days * SECONDS_PER_DAY)) or nil,
        read = h[9] and true or nil,
        returned = h[10] and true or nil,
    }
    if h[8] then message.items = attachments(index) end
    return message
end

local function collectMail()
    local counts = callFirst({ "GetInboxNumItems" })
    local count = counts and plain(counts[1])
    if type(count) ~= "number" then return nil end
    local mail = { capturedAt = time(), messages = {}, totalOnServer = counts and plain(counts[2]) or nil }
    for index = 1, count do
        mail.messages[#mail.messages + 1] = inboxMessage(index)
    end
    return mail
end

ns.Snapshot.addWindowSection("mail", collectMail, { "MAIL_SHOW", "MAIL_INBOX_UPDATE" })
