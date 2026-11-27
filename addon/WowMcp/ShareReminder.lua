-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- The sharing reminder: a rare chat line that offers the sharing window, the way QuestieTrace asks for its data.
--
-- It is asked only for a class, client build and language this account has not reported yet, never before the
-- character has a saved dump, and at most once in REMIND_EVERY; "I reported this" in the
-- window ends it for that combination, and /mcp share off (or the options page) ends it for good. It prints one
-- line and nothing else: no window opens and nothing is sent unless the player clicks.

local _, ns = ...
local Store, ShareReport, ChatLinks, chat = ns.Store, ns.ShareReport, ns.ChatLinks, ns.chat

local ShareReminder = {}
ns.ShareReminder = ShareReminder

ShareReminder.REMIND_EVERY = 24 * 3600
local LOGIN_DELAY_SECONDS = 45
local LINK_ACTION, LINK_COLOR = "share", "|cff40ff80"

function ShareReminder.isEnabled()
    return Store.settings().shareReminder ~= false
end

function ShareReminder.setEnabled(enabled)
    local settings = Store.settings()
    if enabled then settings.shareReminder = nil else settings.shareReminder = false end
end

-- What the account has been asked and has reported so far; state is the settings.share table (may be nil).
-- Returns whether a reminder for this signature is due at `now`.
function ShareReminder.due(state, signature, now, enabled)
    if not enabled or not signature then return false end
    state = type(state) == "table" and state or {}
    if type(state.reported) == "table" and state.reported[signature] then return false end
    local last = tonumber(state.remindedAt) or 0
    return now - last >= ShareReminder.REMIND_EVERY
end

local function state()
    local settings = Store.settings()
    if type(settings.share) ~= "table" then settings.share = {} end
    return settings.share
end

local function count(map, signature)
    return type(map) == "table" and tonumber(map[signature]) or 0
end

local function message()
    local offer = ChatLinks.enabled() and ChatLinks.text(LINK_ACTION, "open", "Click here to open the export window", LINK_COLOR)
        or "Type /mcp share to open the export window"
    return "Only a few classes and clients have been tested with WowMcp. A short report with no names in it helps to support yours. "
        .. offer .. ". Turn this reminder off: /mcp share off"
end

-- Prints the reminder now, whatever the limits say and without counting it (/mcp share remind).
function ShareReminder.show()
    chat(message())
end

-- Prints the reminder when one is due. Returns whether it did.
function ShareReminder.check()
    local signature = ShareReport.signature()
    if not ShareReminder.due(Store.settings().share, signature, time(), ShareReminder.isEnabled()) then return false end
    local share = state()
    share.remindedAt = time()
    if type(share.asked) ~= "table" then share.asked = {} end
    share.asked[signature] = count(share.asked, signature) + 1
    chat(message())
    return true
end

-- The player says the report for the current class, client and language is sent: no more reminders for it.
-- A new client build or another class is a new combination, so there is something new to ask for.
local function markReported()
    local signature = ShareReport.signature()
    if not signature then return end
    local share = state()
    if type(share.reported) ~= "table" then share.reported = {} end
    share.reported[signature] = time()
end

-- Some time after login, so the line is not lost among the login messages. A client without timers gets no
-- reminder; /mcp share works everywhere.
function ShareReminder.schedule()
    local timer = _G.C_Timer
    if type(timer) ~= "table" or type(timer.After) ~= "function" then return false end
    timer.After(LOGIN_DELAY_SECONDS, function() pcall(ShareReminder.check) end)
    return true
end

ns.ShareWindow.onReported(markReported)
ChatLinks.on(LINK_ACTION, function()
    ns.ShareWindow.open()
    return true
end)
