-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Client API compatibility layer: the only place that knows that functions
-- have different names in different clients and that Forever has "secrets".
-- Sections get plain values from here.

local _, ns = ...

local Compat = {}
ns.Compat = Compat

-- Only plain values go into SavedVariables. The Forever client has "secrets"
-- (values the addon cannot read in combat); we do not save those.
function Compat.plain(value)
    local t = type(value)
    if t ~= "string" and t ~= "number" and t ~= "boolean" then return nil end
    if issecretvalue and issecretvalue(value) then return nil end
    return value
end

local plain = Compat.plain

-- Deepest table nesting written to SavedVariables; a deeper table is dropped (real dumps have under ten levels).
Compat.MAX_SAVED_DEPTH = 20

local function isPlainKey(key)
    local t = type(key)
    if t ~= "string" and t ~= "number" then return false end
    return not (issecretvalue and issecretvalue(key))
end

-- Length of a table that is a list 1..n (no other keys, no holes); nil for any other table.
local function listLength(tbl)
    local count = 0
    for key in pairs(tbl) do
        if not isPlainKey(key) or type(key) ~= "number" or key < 1 or key % 1 ~= 0 then return nil end
        count = count + 1
    end
    for i = 1, count do
        if tbl[i] == nil then return nil end
    end
    return count
end

-- Removes, in place and at any depth, everything that must not reach SavedVariables: secret values and keys,
-- functions, userdata and tables nested deeper than MAX_SAVED_DEPTH. A list stays a list (gaps are closed), so the
-- server never sees a list turn into a map. Every write to SavedVariables ends with this pass (Snapshot.sanitizeCurrent),
-- so a value a section forgot to pass through plain() cannot be written. Returns how many entries were removed.
function Compat.sanitizeTree(tbl, depth)
    depth = depth or 1
    local length = listLength(tbl)
    local removedHere, removed = 0, 0
    for key, value in pairs(tbl) do
        local keep
        if not isPlainKey(key) then
            keep = false
        elseif type(value) == "table" then
            keep = depth < Compat.MAX_SAVED_DEPTH
            if keep then removed = removed + Compat.sanitizeTree(value, depth + 1) end
        else
            keep = plain(value) ~= nil
        end
        if not keep then
            tbl[key] = nil -- assigning nil to an existing field is allowed during pairs
            removedHere = removedHere + 1
        end
    end
    if length and removedHere > 0 then
        local j = 0
        for i = 1, length do
            if tbl[i] ~= nil then
                j = j + 1
                tbl[j] = tbl[i]
            end
        end
        for i = j + 1, length do tbl[i] = nil end
    end
    return removed + removedHere
end

function Compat.plainFields(source)
    local out = {}
    for key, value in pairs(source) do
        if type(key) == "string" and plain(value) ~= nil then
            out[key] = value
        end
    end
    return out
end

-- Picks one of two values by a condition. Instead of `cond and a or b`, which
-- substitutes `b` when `a` is nil (the neighbouring field), i.e. returns the wrong value.
function Compat.pick(condition, whenTrue, whenFalse)
    if condition then return whenTrue end
    return whenFalse
end

-- Returns the function at a path like "C_PetInfo.GetPetHappiness", or nil.
function Compat.resolve(path)
    local object = _G
    for part in path:gmatch("[^.]+") do
        if type(object) ~= "table" then return nil end
        object = object[part]
    end
    return type(object) == "function" and object or nil
end

-- Calls the first existing function from a list of paths. Returns the result table
-- and the path that worked, or nil if none exists or every one raises an error.
function Compat.callFirst(paths, ...)
    for _, path in ipairs(paths) do
        local fn = Compat.resolve(path)
        if fn then
            local results = { pcall(fn, ...) }
            if results[1] then
                table.remove(results, 1)
                return results, path
            end
        end
    end
    return nil
end

-- First value returned by the first working function, already filtered. A boolean false stays false
-- (`results and plain(x) or nil` would turn it into nil, e.g. UnitIsConnected for an offline group member).
function Compat.firstValue(paths, ...)
    local results = Compat.callFirst(paths, ...)
    if not results then return nil end
    return plain(results[1])
end

-- Strips color and icon codes used in item descriptions from the text.
function Compat.stripMarkup(text)
    text = text:gsub("|c%x%x%x%x%x%x%x%x", ""):gsub("|r", "")
    text = text:gsub("|T.-|t", ""):gsub("|A.-|a", "")
    return text
end

-- Converts tooltip data (C_TooltipInfo) into a list of text lines.
local function tooltipLines(data)
    if type(data) ~= "table" or type(data.lines) ~= "table" then return nil end
    local out = {}
    for _, line in ipairs(data.lines) do
        local left, right = plain(line.leftText), plain(line.rightText)
        if type(left) == "string" then
            left = Compat.stripMarkup(left)
            if left:find("%S") then
                if type(right) == "string" and right:find("%S") then
                    out[#out + 1] = left .. " | " .. Compat.stripMarkup(right)
                else
                    out[#out + 1] = left
                end
            end
        end
    end
    if #out == 0 then return nil end
    return out
end

-- Tooltip lines from C_TooltipInfo[method](...), or nil if the client lacks that function.
function Compat.tooltipFor(method, ...)
    if not (C_TooltipInfo and type(C_TooltipInfo[method]) == "function") then return nil end
    local ok, data = pcall(C_TooltipInfo[method], ...)
    if ok then return tooltipLines(data) end
    return nil
end
