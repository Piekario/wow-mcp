-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Game events: one frame and an event -> handlers table.
-- Modules register through Events.on instead of knowing one shared table.
--
-- Each handler runs in pcall: an error in one (e.g. the vendor's) does not stop the next
-- ones for the same event (e.g. the buyback list). Errors go to Events.errors(),
-- and the dump appends them to the errors field so they are visible without the Lua error popup.

local _, ns = ...

local Events = {}
ns.Events = Events

local MAX_ERRORS = 10

local handlersByEvent = {}
local registration = {} -- event -> whether the client accepted RegisterEvent
local handlerErrors = {} -- "event: error" strings from this session (no repeats), at most MAX_ERRORS
local seenErrors = {}
local frame = CreateFrame("Frame")

local function rememberError(event, err)
    local text = "event " .. tostring(event) .. ": " .. tostring(err)
    if seenErrors[text] or #handlerErrors >= MAX_ERRORS then return end
    seenErrors[text] = true
    handlerErrors[#handlerErrors + 1] = text
end

frame:SetScript("OnEvent", function(_, event, ...)
    local handlers = handlersByEvent[event]
    if not handlers then return end
    for _, handler in ipairs(handlers) do
        local ok, err = pcall(handler, ...)
        if not ok then rememberError(event, err) end
    end
end)

function Events.on(event, handler)
    if not handlersByEvent[event] then
        handlersByEvent[event] = {}
        -- An unknown event name raises an error in new clients, hence pcall.
        registration[event] = pcall(frame.RegisterEvent, frame, event) and true or false
    end
    local handlers = handlersByEvent[event]
    handlers[#handlers + 1] = handler
end

-- Copy of each addon event's registration result (true = the client accepted it); for /mcp probe.
function Events.registrationResults()
    local copy = {}
    for event, accepted in pairs(registration) do copy[event] = accepted end
    return copy
end

-- Copy of event handler errors from this session (since login or /reload).
function Events.errors()
    local copy = {}
    for i, text in ipairs(handlerErrors) do copy[i] = text end
    return copy
end
