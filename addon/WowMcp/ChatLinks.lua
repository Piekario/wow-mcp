-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Clickable links in WowMcp chat lines: "|Hwowmcp:<action>:<argument>|h[text]|h". One link type for the whole
-- addon, and each feature registers its action (way: a TomTom waypoint, share: the sharing window).
--
-- Uses the client's link handler API (LinkUtil.RegisterLinkHandler, which Questie also uses on Forever): a
-- handler, not a replaced function, so no Blizzard code is tainted. A modified click (shift-click) keeps its
-- default. Without that API there are no links and the callers print plain text.

local _, ns = ...

local ChatLinks = {}
ns.ChatLinks = ChatLinks

local LINK_TYPE = "wowmcp"
local actions = {}
local enabled = false

function ChatLinks.enabled()
    return enabled
end

-- handler(link) runs when the player clicks a link of this action.
function ChatLinks.on(action, handler)
    actions[action] = handler
end

function ChatLinks.text(action, argument, label, color)
    return ("%s|H%s:%s:%s|h[%s]|h|r"):format(color, LINK_TYPE, action, argument or "", label)
end

-- Runs the action of a clicked link. Returns true when the link was ours.
function ChatLinks.click(link)
    if type(link) ~= "string" then return false end
    local action = link:match("^" .. LINK_TYPE .. ":(%a+)")
    local handler = action and actions[action]
    if not handler then return false end
    return handler(link) and true or false
end

local function response(name, default)
    local responses = _G.LinkProcessorResponse
    return (type(responses) == "table" and responses[name] ~= nil) and responses[name] or default
end

local function register()
    local linkUtil = _G.LinkUtil
    if type(linkUtil) ~= "table" or type(linkUtil.RegisterLinkHandler) ~= "function" then return false end
    local ok = pcall(linkUtil.RegisterLinkHandler, LINK_TYPE, function(link)
        local modified = _G.IsModifiedClick
        if type(modified) == "function" and modified() then return response("Unhandled", false) end
        if ChatLinks.click(link) then return response("Handled", true) end
        return response("Unhandled", false)
    end)
    return ok
end

enabled = register()
