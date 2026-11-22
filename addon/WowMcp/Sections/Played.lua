-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section played: played time from /played. The client supplies it in the TIME_PLAYED_MSG event; the addon requests
-- it once after login (and after every /reload), and on a dump adds the time elapsed since the response.

local _, ns = ...
local plain, callFirst = ns.Compat.plain, ns.Compat.callFirst

local Played = {}
ns.Played = Played

local playedInfo -- { total, level, at } from the client's last response

-- Request for played time (/played). The response arrives in TIME_PLAYED_MSG and also appears in chat.
function Played.request()
    if not Played.isTracking() then return end
    callFirst({ "RequestTimePlayed" })
end

-- Whether played time is requested after login (on by default; /mcp played on|off, the options page).
function Played.isTracking()
    return ns.Store.settings().played ~= false
end

-- Only turning it off is saved. Turning it on requests played time right away.
function Played.setTracking(enabled)
    if enabled then
        ns.Store.settings().played = nil
        Played.request()
    else
        ns.Store.settings().played = false
    end
end

local function onTimePlayed(total, level)
    if type(plain(total)) == "number" and type(plain(level)) == "number" then
        playedInfo = { total = total, level = level, at = time() }
    end
end

local function collectPlayed()
    if not playedInfo then return nil end
    local elapsed = time() - playedInfo.at
    return {
        totalSeconds = playedInfo.total + elapsed,
        levelSeconds = playedInfo.level + elapsed,
    }
end

ns.Snapshot.addSection("played", collectPlayed)
ns.Events.on("TIME_PLAYED_MSG", onTimePlayed)
