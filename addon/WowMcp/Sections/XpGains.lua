-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section xpGains: the player's own experience since the previous dump saved to disk, broken down
-- into kills, quests and the rest (exploration, discoveries etc.), with event counts.
-- Reads only CHAT_MSG_COMBAT_XP_GAIN messages, which the client shows in chat anyway
-- (own only), and the quest turn-in event. Lifecycle as in the loot section.
-- Diagnostics: messagesSeen, messagesParsed, messagesSecret (content hidden by the client) and barXp,
-- i.e. XP computed independently from the experience bar (UnitXP on PLAYER_XP_UPDATE). When barXp
-- grows while total stands still, messages do not match the patterns or are hidden.
-- Closed periods with XP are carried in xpGains.periods (oldest first, bounded; see Snapshot.closePeriod).

local _, ns = ...
local plain = ns.Compat.plain

local XpGains = {}
ns.XpGains = XpGains

-- Window (in seconds) in which a quest XP message and the quest turn-in event are treated as the same thing.
XpGains.MATCH_WINDOW = 3

-- Game strings: with a creature name ("%s dies, you gain %d experience.") it is a kill, without a name
-- ("You gain %d experience.") it is a quest or something else. Many variants (bonus, group, raid).
local KILL_GLOBALS = {
    "COMBATLOG_XPGAIN_EXHAUSTION1", "COMBATLOG_XPGAIN_EXHAUSTION1_GROUP", "COMBATLOG_XPGAIN_EXHAUSTION1_RAID",
    "COMBATLOG_XPGAIN_EXHAUSTION2", "COMBATLOG_XPGAIN_EXHAUSTION2_GROUP", "COMBATLOG_XPGAIN_EXHAUSTION2_RAID",
    "COMBATLOG_XPGAIN_EXHAUSTION4", "COMBATLOG_XPGAIN_EXHAUSTION4_GROUP", "COMBATLOG_XPGAIN_EXHAUSTION4_RAID",
    "COMBATLOG_XPGAIN_EXHAUSTION5", "COMBATLOG_XPGAIN_EXHAUSTION5_GROUP", "COMBATLOG_XPGAIN_EXHAUSTION5_RAID",
    "COMBATLOG_XPGAIN_FIRSTPERSON", "COMBATLOG_XPGAIN_FIRSTPERSON_GROUP", "COMBATLOG_XPGAIN_FIRSTPERSON_RAID",
}
local OTHER_GLOBALS = {
    "COMBATLOG_XPGAIN_EXHAUSTION1_UNNAMED", "COMBATLOG_XPGAIN_EXHAUSTION2_UNNAMED",
    "COMBATLOG_XPGAIN_EXHAUSTION4_UNNAMED", "COMBATLOG_XPGAIN_EXHAUSTION5_UNNAMED",
    "COMBATLOG_XPGAIN_FIRSTPERSON_UNNAMED", "COMBATLOG_XPGAIN_FIRSTPERSON_UNNAMED_GROUP",
    "COMBATLOG_XPGAIN_FIRSTPERSON_UNNAMED_RAID", "COMBATLOG_XPGAIN_QUEST",
}

-- When the client lacks these strings: English forms.
local ENGLISH_KILL = { "%s dies, you gain %d experience." }
local ENGLISH_OTHER = { "You gain %d experience." }

local xpSince -- time of the dump that last reached the disk (or of login)
local recentOther -- { xp, at }: the latest "nameless" XP that may turn out to be quest XP
local pendingQuest -- { xp, at }: quest turned in, and the XP message has not arrived yet
local bar -- { level, xp, xpMax }: last experience bar read

local function compile(globals, english, kind)
    local patterns = {}
    for _, global in ipairs(globals) do
        local format = _G[global]
        if type(format) == "string" and format ~= "" then
            local pattern, order = ns.Loot.patternFrom(format)
            patterns[#patterns + 1] = { pattern = pattern, order = order, kind = kind }
        end
    end
    return patterns, english
end

local function xpPatterns()
    local list = {}
    local kills, killEnglish = compile(KILL_GLOBALS, ENGLISH_KILL, "kill")
    local others, otherEnglish = compile(OTHER_GLOBALS, ENGLISH_OTHER, "other")
    for _, p in ipairs(kills) do list[#list + 1] = p end
    for _, p in ipairs(others) do list[#list + 1] = p end
    -- The English forms only when the client has no XP strings at all: on a localized client its own
    -- strings are complete, and an English pattern could only misread a message.
    if #list > 0 then return list end
    if #kills == 0 then
        for _, format in ipairs(killEnglish) do
            local pattern, order = ns.Loot.patternFrom(format)
            list[#list + 1] = { pattern = pattern, order = order, kind = "kill" }
        end
    end
    if #others == 0 then
        for _, format in ipairs(otherEnglish) do
            local pattern, order = ns.Loot.patternFrom(format)
            list[#list + 1] = { pattern = pattern, order = order, kind = "other" }
        end
    end
    return list
end

-- Whether the client has its own XP format strings; then they are the only patterns used (in the client's
-- language), and the English fallbacks below never apply.
function XpGains.usingGameFormats()
    for _, list in ipairs({ KILL_GLOBALS, OTHER_GLOBALS }) do
        for _, global in ipairs(list) do
            if type(_G[global]) == "string" and _G[global] ~= "" then return true end
        end
    end
    return false
end

-- Last resort only when the client has no XP format strings at all (another version): English
-- message beginnings, without the bonus tail.
local LOOSE_PATTERNS = {
    { pattern = "^.+ dies, you gain (%d+) experience", kind = "kill" },
    { pattern = "^You gain (%d+) experience", kind = "other" },
}

-- Kind ("kill" or "other") and number of points from the player's own XP message; nil for other texts.
function XpGains.parse(message, patterns)
    if type(message) ~= "string" then return nil end
    for _, p in ipairs(patterns or xpPatterns()) do
        local captures = { message:match(p.pattern) }
        if #captures > 0 then
            for i, field in ipairs(p.order) do
                if field == "count" then
                    local xp = tonumber(captures[i])
                    if xp then return p.kind, xp end
                end
            end
        end
    end
    if XpGains.usingGameFormats() then return nil end
    for _, p in ipairs(LOOSE_PATTERNS) do
        local xp = tonumber(message:match(p.pattern))
        if xp then return p.kind, xp end
    end
    return nil
end

local function newSection(since)
    return {
        since = since, total = 0,
        kills = { count = 0, xp = 0 }, quests = { count = 0, xp = 0 }, other = { count = 0, xp = 0 },
        messagesSeen = 0, messagesParsed = 0, messagesSecret = 0, barXp = 0,
    }
end

local function xpSection()
    local current = ns.Snapshot.current()
    if type(current.xpGains) ~= "table" then current.xpGains = newSection(xpSince or time()) end
    return current.xpGains
end

-- Diagnostic counter (older dumps may not have it).
local function bump(field, amount)
    local section = xpSection()
    section[field] = (type(section[field]) == "number" and section[field] or 0) + (amount or 1)
end

local function add(bucket, xp)
    local section = xpSection()
    section[bucket].count = section[bucket].count + 1
    section[bucket].xp = section[bucket].xp + xp
    section.total = section.total + xp
    section.lastAt = time()
end

-- Reverts an earlier crediting to "other" (the message arrived before the turn-in event).
local function removeFromOther(xp)
    local section = xpSection()
    local other = section.other
    if other.count > 0 then
        other.count = other.count - 1
        other.xp = math.max(other.xp - xp, 0)
        section.total = math.max(section.total - xp, 0)
    end
end

function XpGains.record(kind, xp)
    local now = time()
    if kind == "kill" then
        add("kills", xp)
        return
    end
    -- "Nameless" XP right after a quest turn-in is quest XP that the turn-in event has already counted.
    if pendingQuest and pendingQuest.xp == xp and now - pendingQuest.at <= XpGains.MATCH_WINDOW then
        pendingQuest = nil
        return
    end
    add("other", xp)
    recentOther = { xp = xp, at = now }
end

function XpGains.recordQuest(xp)
    if type(xp) ~= "number" or xp <= 0 then return end
    local now = time()
    if recentOther and recentOther.xp == xp and now - recentOther.at <= XpGains.MATCH_WINDOW then
        removeFromOther(xp)
        recentOther = nil
    else
        pendingQuest = { xp = xp, at = now }
    end
    add("quests", xp)
end

local function onXpMessage(message)
    bump("messagesSeen")
    local text = plain(message)
    if message ~= nil and text == nil then
        bump("messagesSecret") -- the client hid the content ("secret"), it cannot be read
        return
    end
    local kind, xp = XpGains.parse(text)
    if kind then
        bump("messagesParsed")
        XpGains.record(kind, xp)
    end
end

local function readBar()
    local level, xp, xpMax = plain(UnitLevel("player")), plain(UnitXP("player")), plain(UnitXPMax("player"))
    if type(level) ~= "number" or type(xp) ~= "number" or type(xpMax) ~= "number" then return nil end
    return { level = level, xp = xp, xpMax = xpMax }
end

-- XP between two bar reads; a one-level level-up adds the remainder of the previous bar.
function XpGains.barDelta(before, after)
    if not (before and after) then return 0 end
    if after.level == before.level then return math.max(after.xp - before.xp, 0) end
    if after.level == before.level + 1 then return math.max(before.xpMax - before.xp, 0) + after.xp end
    return 0 -- jump of several levels or a drop: we do not guess
end

local function onXpUpdate(unit)
    if unit ~= nil and unit ~= "player" then return end
    local now = readBar()
    local delta = XpGains.barDelta(bar, now)
    if now then bar = now end
    if delta > 0 then bump("barXp", delta) end
end

-- QUEST_TURNED_IN(questID, xpReward, moneyReward)
local function onQuestTurnedIn(_, xp)
    XpGains.recordQuest(plain(xp))
end

-- A closed period is worth carrying only when experience came in (from messages or the bar).
function XpGains.hasContent(section)
    return (type(section.total) == "number" and section.total > 0) or (type(section.barXp) == "number" and section.barXp > 0)
end

-- After login (and after /reload) the previous dump is already on disk together with its XP; the closed period
-- is carried in xpGains.periods (Snapshot.startPeriod).
local function startXpPeriod()
    xpSince = ns.Snapshot.startPeriod("xpGains", newSection, XpGains.hasContent)
    bar = readBar()
end

ns.Snapshot.addWindowSection("xpGains", nil, nil, { quiet = true })
ns.Events.on("PLAYER_LOGIN", function() pcall(startXpPeriod) end)
ns.Events.on("CHAT_MSG_COMBAT_XP_GAIN", ns.Snapshot.whenEnabled("xpGains", function(message) pcall(onXpMessage, message) end))
ns.Events.on("QUEST_TURNED_IN", ns.Snapshot.whenEnabled("xpGains", function(...) pcall(onQuestTurnedIn, ...) end))
ns.Events.on("PLAYER_XP_UPDATE", ns.Snapshot.whenEnabled("xpGains", function(unit) pcall(onXpUpdate, unit) end))
