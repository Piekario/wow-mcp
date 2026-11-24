-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- The client report: a few lines about the game client and the last dump that a player can copy out of the game
-- and paste into an issue, so the project learns how the addon behaves for their class, client and language.
--
-- The report is built from a fixed list of fields, never by copying dump contents: it holds no name of a
-- character, realm, guild or other player, no item, no gold and no text the player wrote. The full anonymized
-- dump is made outside the game by the server (wow_share_dump). Data only, no frames.

local _, ns = ...
local Store, Snapshot = ns.Store, ns.Snapshot

local ShareReport = {}
ns.ShareReport = ShareReport

ShareReport.FORMAT = 1
ShareReport.ISSUE_URL = "https://github.com/Piekario/wow-mcp/issues/new?template=share_dump.yml"

local MAX_ERRORS, MAX_ERROR_LENGTH = 10, 200
local META_FIELDS = {
    errors = true, dumpedAt = true, dumpedAtText = true, reason = true, addonVersion = true, timings = true,
    disabledSections = true, lastSkippedLogoutAt = true, clientUnloaded = true,
}

local function text(value, fallback)
    if type(value) == "string" and value ~= "" then return value end
    if type(value) == "number" then return tostring(value) end
    return fallback or "?"
end

local function sortedKeys(map)
    local keys = {}
    for key in pairs(map) do
        if type(key) == "string" then keys[#keys + 1] = key end
    end
    table.sort(keys)
    return keys
end

-- How much a section holds: list items, otherwise keys.
local function size(value)
    if value == nil then return 0 end
    if type(value) ~= "table" then return 1 end
    if #value > 0 then return #value end
    local count = 0
    for _ in pairs(value) do count = count + 1 end
    return count
end

-- Identifies what a report covers, so the player is not asked twice for the same class, client and language.
function ShareReport.signature(snapshot)
    snapshot = snapshot or Store.savedCurrentCharacter()
    local character = type(snapshot) == "table" and snapshot.character
    if type(character) ~= "table" or type(character.classToken) ~= "string" then return nil end
    return character.classToken .. "|" .. text(character.clientBuild) .. "|" .. text(character.locale)
end

local function sectionLines(snapshot)
    local present, seen = {}, {}
    for _, name in ipairs(sortedKeys(snapshot)) do
        if not META_FIELDS[name] then
            present[#present + 1] = name .. "=" .. size(snapshot[name])
            seen[name] = true
        end
    end
    local absent = {}
    for _, name in ipairs(Snapshot.sectionNames()) do
        if not seen[name] and not Snapshot.isDisabled(name) then absent[#absent + 1] = name end
    end
    local lines = { "sections (" .. #present .. "): " .. table.concat(present, ", ") }
    if #absent > 0 then lines[#lines + 1] = "not in the dump: " .. table.concat(absent, ", ") end
    local disabled = Snapshot.disabledNames()
    if #disabled > 0 then lines[#lines + 1] = "disabled: " .. table.concat(disabled, ", ") end
    return lines
end

local function errorLines(snapshot)
    local errors = type(snapshot.errors) == "table" and snapshot.errors or {}
    local lines = { "errors: " .. #errors }
    for index = 1, math.min(#errors, MAX_ERRORS) do
        lines[#lines + 1] = "  - " .. tostring(errors[index]):gsub("%s+", " "):sub(1, MAX_ERROR_LENGTH)
    end
    return lines
end

local function rejectedEvents()
    local rejected = {}
    for event, accepted in pairs(ns.Events.registrationResults()) do
        if not accepted then rejected[#rejected + 1] = event end
    end
    table.sort(rejected)
    return rejected
end

local function probeLine(probe)
    if type(probe) ~= "table" or type(probe.namespaces) ~= "table" then
        return "probe: none (type /mcp probe for the list of client API functions)"
    end
    local parts = {}
    for _, name in ipairs(sortedKeys(probe.namespaces)) do
        parts[#parts + 1] = name .. "=" .. size(probe.namespaces[name])
    end
    return "probe: " .. size(probe.globals) .. " global functions; " .. table.concat(parts, ", ")
end

local function dumpLines(snapshot)
    local c = type(snapshot.character) == "table" and snapshot.character or {}
    local timings = type(snapshot.timings) == "table" and snapshot.timings or {}
    local lines = {
        "client: " .. text(c.clientVersion) .. " build " .. text(c.clientBuild) .. ", interface " .. text(c.clientInterface)
            .. ", locale " .. text(c.locale),
        "character: " .. text(c.classToken) .. ", " .. text(c.raceToken) .. ", " .. text(c.faction) .. ", level " .. text(c.level),
        "dump: reason " .. text(snapshot.reason) .. ", collected in " .. text(timings.totalMs) .. " ms",
    }
    for _, group in ipairs({ sectionLines(snapshot), errorLines(snapshot) }) do
        for _, line in ipairs(group) do lines[#lines + 1] = line end
    end
    local rejected = rejectedEvents()
    if #rejected > 0 then lines[#lines + 1] = "events the client rejected: " .. table.concat(rejected, ", ") end
    lines[#lines + 1] = probeLine(snapshot.probe)
    return lines
end

-- An error text is the one place where the client could quote a name; the character's own is cut out.
local function withoutPlayerName(line)
    local name = UnitName and UnitName("player")
    if type(name) ~= "string" or #name < 2 then return line end
    return (line:gsub(name:gsub("%p", "%%%0"), "Player"))
end

function ShareReport.lines()
    local lines = {
        "WowMcp client report, format " .. ShareReport.FORMAT,
        "addon: " .. ns.ADDON_VERSION .. ", schema " .. ns.SCHEMA_VERSION,
    }
    local snapshot = Store.savedCurrentCharacter()
    if type(snapshot) ~= "table" or type(snapshot.character) ~= "table" then
        lines[#lines + 1] = "no dump yet: type /mcp and open this window again"
        return lines
    end
    for _, line in ipairs(dumpLines(snapshot)) do lines[#lines + 1] = withoutPlayerName(line) end
    return lines
end

-- The report as one line between a start and an end mark, so it fits a one-line box and a paste that lost its
-- end can be told from a whole one.
function ShareReport.text()
    local mark = "WowMcp:" .. ShareReport.FORMAT
    return mark .. "! " .. table.concat(ShareReport.lines(), " || ") .. " !End:" .. mark .. "!"
end

local function probedFunctions(probe)
    if type(probe) ~= "table" or type(probe.namespaces) ~= "table" then return 0 end
    local total = size(probe.globals)
    for _, names in pairs(probe.namespaces) do total = total + size(names) end
    return total
end

-- The numbers shown under the report: { sections, errors, apiFunctions, dumpMs }; nil without a dump.
function ShareReport.counters()
    local snapshot = Store.savedCurrentCharacter()
    if type(snapshot) ~= "table" or type(snapshot.character) ~= "table" then return nil end
    local sections = 0
    for name in pairs(snapshot) do
        if not META_FIELDS[name] then sections = sections + 1 end
    end
    return {
        sections = sections,
        errors = type(snapshot.errors) == "table" and #snapshot.errors or 0,
        apiFunctions = probedFunctions(snapshot.probe),
        dumpMs = type(snapshot.timings) == "table" and snapshot.timings.totalMs or nil,
    }
end
