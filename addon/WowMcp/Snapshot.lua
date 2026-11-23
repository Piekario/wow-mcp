-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Dump of character state and the registry of its sections.
--
-- Two kinds of sections:
--   * collected on every dump (Snapshot.addSection), in registration order,
--     i.e. in file order in WowMcp.toc;
--   * collected when a window opens (Snapshot.addWindowSection): saved
--     immediately in the current dump, and carried over from the previous one on a plain dump.
--     Data recorded continuously (deaths, note) is carried over the same way.
-- A new section = a new file in Sections/ with one registration call.
--
-- The player can disable a section (/mcp off): such a section is not collected and disappears from the dump.

local _, ns = ...
local Store, Events, plain = ns.Store, ns.Events, ns.Compat.plain

local Snapshot = {}
ns.Snapshot = Snapshot

local ALWAYS_ON = "character" -- without it the dump does not say whose it is

local dumpSections = {}       -- { name, collect } in dump order
local windowSections = {}     -- names of window sections (and continuously collected data)
local windowCollectors = {}   -- name -> collect (when the section has one simple read)
local quietSections = {}      -- sections that do not remind about saving to disk
local capturedSinceSave = {}  -- name -> true: window data that is not on disk yet
local histories = {}          -- section name -> { name, key, max } (e.g. vendor -> vendors)
local dumpRefreshers = {}     -- { name, refresh }: window sections that a dump refreshes with a cheap read
local changeListeners = {}
local unsaved = false         -- dump from /mcp without /reload: in memory, but not on disk yet

function Snapshot.addSection(name, collect)
    dumpSections[#dumpSections + 1] = { name = name, collect = collect }
end

-- Window section. With collect and events: each of these events stores the result of collect.
-- Without them the section handles its own events (e.g. recipes, auctions).
-- options.quiet: the data needs no reminder to save (probe, deaths, note).
-- options.history = { name, key, max }: a second section (e.g. vendors) with a result[key] -> result map
-- for the last few windows (e.g. vendors); the earliest visited fall off beyond max.
function Snapshot.addWindowSection(name, collect, events, options)
    windowSections[#windowSections + 1] = name
    windowCollectors[name] = collect
    quietSections[name] = options and options.quiet or nil
    local history = options and options.history
    if history then
        windowSections[#windowSections + 1] = history.name
        histories[name] = history
    end
    for _, event in ipairs(events or {}) do
        Events.on(event, function() Snapshot.capture(name) end)
    end
end

-- Window section that every dump refreshes with a cheap read of client memory (e.g. whether the character is
-- in a guild): refresh(previous value) returns the new one. Never asks the game server for data.
function Snapshot.refreshOnDump(name, refresh)
    dumpRefreshers[#dumpRefreshers + 1] = { name = name, refresh = refresh }
end

-- All section names that can reach the dump, alphabetically.
function Snapshot.sectionNames()
    local names = {}
    for _, section in ipairs(dumpSections) do names[#names + 1] = section.name end
    for _, name in ipairs(windowSections) do names[#names + 1] = name end
    table.sort(names)
    return names
end

-- Canonical section name for player input (case-insensitive), or nil.
function Snapshot.resolveName(text)
    local wanted = tostring(text or ""):lower()
    for _, name in ipairs(Snapshot.sectionNames()) do
        if name:lower() == wanted then return name end
    end
    return nil
end

function Snapshot.canDisable(name)
    return name ~= ALWAYS_ON
end

function Snapshot.isDisabled(name)
    return Snapshot.canDisable(name) and Store.isSectionDisabled(name)
end

-- Enables or disables a section. A disabled one also disappears from the current in-memory dump.
function Snapshot.setEnabled(name, enabled)
    Store.setSectionDisabled(name, not enabled)
    if not enabled then
        Snapshot.current()[name] = nil
        capturedSinceSave[name] = nil
    end
end

-- Names of disabled sections (only existing ones), alphabetically.
function Snapshot.disabledNames()
    local names = {}
    for _, name in ipairs(Snapshot.sectionNames()) do
        if Snapshot.isDisabled(name) then names[#names + 1] = name end
    end
    return names
end

-- Wraps a window section's event handler: a disabled section reads nothing.
function Snapshot.whenEnabled(name, handler)
    return function(...)
        if Snapshot.isDisabled(name) then return end
        return handler(...)
    end
end

-- Dump of the character being played (a place for window section results).
Snapshot.current = Store.currentCharacter

-- The panel (and any other view) registers here to refresh after new window data.
function Snapshot.onWindowCaptured(listener)
    changeListeners[#changeListeners + 1] = listener
end

-- Sections filled by windows that are not on disk yet (only the player saves).
function Snapshot.pendingWindowSections()
    local names = {}
    for name in pairs(capturedSinceSave) do names[#names + 1] = name end
    table.sort(names)
    return names
end

-- A window section has just stored data in the current dump.
function Snapshot.windowCaptured(name)
    if quietSections[name] then return end
    capturedSinceSave[name] = true
    for _, listener in ipairs(changeListeners) do pcall(listener) end
end

-- Keeps at most max entries in the map; those with the oldest capturedAt fall off
-- (on equal times the key decides, so the result is reproducible).
function Snapshot.keepNewest(map, max)
    local keys = {}
    for key in pairs(map) do keys[#keys + 1] = key end
    if #keys <= max then return end
    table.sort(keys, function(a, b)
        local ta, tb = map[a].capturedAt or 0, map[b].capturedAt or 0
        if ta ~= tb then return ta > tb end
        return tostring(a) < tostring(b)
    end)
    for i = max + 1, #keys do map[keys[i]] = nil end
end

-- Appends a window result to a section with history (e.g. vendors), unless that one is disabled.
-- The key (NPC name) goes through plain: a client "secret" cannot be a table key
-- nor be compared, so without a plain value no history entry is created.
local function rememberInHistory(name, result)
    local history = histories[name]
    if not history or Snapshot.isDisabled(history.name) then return end
    local key = plain(result[history.key])
    if type(key) ~= "string" or key == "" then return end
    local current = Snapshot.current()
    if type(current[history.name]) ~= "table" then current[history.name] = {} end
    current[history.name][key] = result
    Snapshot.keepNewest(current[history.name], history.max)
    -- The reminder to save is already handled by the main section (vendor, trainer).
end

-- Stores a window section's result in the character's current dump.
function Snapshot.capture(name)
    if Snapshot.isDisabled(name) or not windowCollectors[name] then return end
    local ok, result = pcall(windowCollectors[name])
    if ok and result then
        Snapshot.current()[name] = result
        Snapshot.windowCaptured(name)
        pcall(rememberInHistory, name, result)
    end
end

-- Whether the saved dump ends with a skipped logout dump (the client already had empty data):
-- then the disk holds an old dump, and the loot and XP period continues from it.
function Snapshot.continuesPeriod(saved)
    return type(saved) == "table" and type(saved.lastSkippedLogoutAt) == "number"
        and type(saved.dumpedAt) == "number" and saved.lastSkippedLogoutAt >= saved.dumpedAt
end

-- Closed periods. The addon never assumes the server read a dump: the game may save twice
-- (/mcp reload, then logout) before the server looks at the file, and the second save would otherwise
-- replace the first period's loot and XP. So when a login closes a period that has content, the period
-- (its fields plus until = the dumpedAt of the save that closed it) is appended to the section's
-- "periods" list, oldest first, and carried on. The list is bounded by count and by age; empty periods
-- are not stored.
Snapshot.MAX_PERIODS = 20
Snapshot.MAX_PERIOD_AGE = 7 * 24 * 3600

-- The carried periods that are still within the bounds at time now: tables with a numeric until not older
-- than MAX_PERIOD_AGE, at most MAX_PERIODS of the newest. Returns a new list (possibly empty).
function Snapshot.keptPeriods(periods, now)
    local kept = {}
    if type(periods) ~= "table" then return kept end
    for _, period in ipairs(periods) do
        local ended = type(period) == "table" and period["until"]
        if type(ended) == "number" and now - ended <= Snapshot.MAX_PERIOD_AGE then kept[#kept + 1] = period end
    end
    while #kept > Snapshot.MAX_PERIODS do table.remove(kept, 1) end
    return kept
end

-- Closes the saved section's period at time ended: returns the carried list with the closed period appended
-- when hasContent(section) says it holds anything, bounded by keptPeriods; nil when the list is empty.
function Snapshot.closePeriod(section, ended, hasContent, now)
    if type(section) ~= "table" then return nil end
    local periods = section.periods
    section.periods = nil
    if type(section.since) == "number" and hasContent(section) then
        section["until"] = ended
        periods = type(periods) == "table" and periods or {}
        periods[#periods + 1] = section
    end
    local kept = Snapshot.keptPeriods(periods, now)
    if #kept == 0 then return nil end
    return kept
end

-- After login (and after /reload) starts a new period of section name: the previous dump is already
-- on disk, so the section starts from zero with since = its dumpedAt. fresh(since) creates an empty section,
-- and the closed period goes to the section's periods list when hasContent(section) is true.
-- When the previous logout dump was skipped, the period continues (with its periods). Returns since or nil.
function Snapshot.startPeriod(name, fresh, hasContent)
    local saved = Store.savedCurrentCharacter()
    local previous = saved and type(saved[name]) == "table" and saved[name] or nil
    if Snapshot.continuesPeriod(saved) and previous and type(previous.since) == "number" then
        local kept = Snapshot.keptPeriods(previous.periods, time())
        previous.periods = #kept > 0 and kept or nil
        return previous.since
    end
    local since = (saved and type(saved.dumpedAt) == "number" and saved.dumpedAt) or time()
    if Snapshot.isDisabled(name) then
        if saved then saved[name] = nil end
        return since
    end
    local section = fresh(since)
    section.periods = Snapshot.closePeriod(previous, since, hasContent, time())
    Snapshot.current()[name] = section
    return since
end

local function collectSections(snapshot, recorder)
    for _, section in ipairs(dumpSections) do
        if not Snapshot.isDisabled(section.name) then
            local ok, result = recorder.measure(section.name, section.collect)
            if ok then
                snapshot[section.name] = result
            else
                snapshot.errors[#snapshot.errors + 1] = section.name .. ": " .. tostring(result)
            end
        end
    end
end

local function keepWindowSections(snapshot, previous)
    for _, name in ipairs(windowSections) do
        if not Snapshot.isDisabled(name) then
            snapshot[name] = previous and previous[name] or nil
        end
    end
    for _, r in ipairs(dumpRefreshers) do
        if not Snapshot.isDisabled(r.name) then
            local ok, value = pcall(r.refresh, snapshot[r.name])
            if ok then
                snapshot[r.name] = value
            else
                snapshot.errors[#snapshot.errors + 1] = r.name .. ": " .. tostring(value)
            end
        end
    end
end

local function isEmptyList(value)
    return type(value) == "table" and next(value) == nil
end

local function hasNoBagSpace(bags)
    if type(bags) ~= "table" then return false end
    for _, bag in ipairs(bags) do
        if type(bag) == "table" and type(bag.size) == "number" and bag.size > 0 then return false end
    end
    return true
end

-- The four sections whose emptiness says the client has unloaded the character. The guard reads them
-- even when the player turned them off with /mcp off (see guardView), so it never depends on that choice.
local GUARD_SECTIONS = { "stats", "spells", "equipment", "bags" }

-- How many empty-client signs the dump has.
function Snapshot.unloadedSignals(snapshot)
    local signals = 0
    if type(snapshot.stats) == "table" and not (type(snapshot.stats.healthMax) == "number" and snapshot.stats.healthMax > 0) then
        signals = signals + 1
    end
    if isEmptyList(snapshot.spells) then signals = signals + 1 end
    if isEmptyList(snapshot.equipment) then signals = signals + 1 end
    if hasNoBagSpace(snapshot.bags) then signals = signals + 1 end
    return signals
end

local function collectorOf(name)
    for _, section in ipairs(dumpSections) do
        if section.name == name then return section.collect end
    end
    return nil
end

-- The sections the guard looks at: the dump's own, and for a disabled one a fresh read that is only used for the
-- check (never stored in the dump). A section that errored simply gives no sign, as before.
local function guardView(snapshot)
    local view = {}
    for _, name in ipairs(GUARD_SECTIONS) do
        view[name] = snapshot[name]
        local collect = collectorOf(name)
        if view[name] == nil and collect and Snapshot.isDisabled(name) then
            local ok, result = pcall(collect)
            if ok then view[name] = result end
        end
    end
    return view
end

-- On a real exit from the game (and logout to the menu) PLAYER_LOGOUT arrives once the client has
-- already unloaded the character: zero health, no spells, equipment or bags. Two signs are enough;
-- a real character has none (it always has a backpack, health and Attack in the spellbook).
-- Independent of /mcp off: a disabled sign section is read for the check only.
function Snapshot.looksUnloaded(snapshot)
    return Snapshot.unloadedSignals(guardView(snapshot)) >= 2
end

-- Last pass before the game writes SavedVariables: drops from the character's saved dump anything that is not a
-- plain value (Compat.sanitizeTree). Runs at the end of every dump and once more at logout, after window data
-- captured since the last dump.
function Snapshot.sanitizeCurrent()
    local saved = Store.savedCurrentCharacter()
    if type(saved) == "table" then ns.Compat.sanitizeTree(saved) end
end

local function appendEventErrors(snapshot)
    for _, text in ipairs(Events.errors()) do
        snapshot.errors[#snapshot.errors + 1] = text
    end
end

-- Collects all enabled sections and saves the character's dump in WowMcpDB.
-- A logout dump from an empty client does not overwrite the previous one: that one stays (with window data,
-- loot and XP collected since then) and only gets the lastSkippedLogoutAt marker.
-- Returns the saved dump and true when the new one was skipped.
function Snapshot.dump(reason)
    local key = Store.characterKey()
    local previous = Store.openCharacters()[key]
    local snapshot = { errors = {} }
    local recorder = ns.Timing.newRecorder()
    collectSections(snapshot, recorder)
    snapshot.timings = recorder.report()

    if reason == "logout" and Snapshot.looksUnloaded(snapshot) then
        if type(previous) == "table" and type(previous.dumpedAt) == "number" then
            previous.lastSkippedLogoutAt = time()
            Store.saveCharacter(key, previous)
            pcall(Snapshot.sanitizeCurrent)
            return previous, true
        end
        snapshot.clientUnloaded = true -- nothing to keep: we save, but with a warning
    end

    appendEventErrors(snapshot)
    keepWindowSections(snapshot, previous)

    local disabled = Snapshot.disabledNames()
    snapshot.disabledSections = #disabled > 0 and disabled or nil
    snapshot.dumpedAt = time()
    snapshot.dumpedAtText = date("%Y-%m-%d %H:%M:%S")
    snapshot.reason = reason
    snapshot.addonVersion = ns.ADDON_VERSION

    Store.saveCharacter(key, snapshot)
    pcall(Snapshot.sanitizeCurrent)
    return snapshot, false
end

function Snapshot.markUnsaved()
    unsaved = true
end

function Snapshot.isUnsaved()
    return unsaved
end
