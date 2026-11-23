-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Discovery of the client's Legacy API (see the legacy section): Enum entries, C_* namespaces (named "Legacy" or
-- "Challenge"), functions and frames whose name contains "Legacy", and the results of the namespaces'
-- argument-free Get/Is/Has/Are functions, so a later version can use the real API. The scan of _G is costly
-- (78 ms of a 112 ms real dump), so the names are found once per session and again only after the client loads an
-- addon with "Legacy" in its name (the Legacy window is load-on-demand); the readings and frame fields are fresh
-- on every dump.

local _, ns = ...
local plain = ns.Compat.plain

local LegacyDiscovery = {}
ns.LegacyDiscovery = LegacyDiscovery

local MAX_DISCOVERED = 60       -- entries per discovery list
local MAX_COPY_ENTRIES = 40     -- entries per table copied from a reading
local MAX_COPY_DEPTH = 3        -- nesting of a copied reading
local READ_PREFIXES = { "^Get", "^Is", "^Has", "^Are" }

local function mentionsLegacy(name)
    return type(name) == "string" and name:lower():find("legacy", 1, true) ~= nil
end

local function mentionsChallenge(name)
    return type(name) == "string" and name:lower():find("challenge", 1, true) ~= nil
end

local function append(list, value, limit)
    if #list < (limit or MAX_DISCOVERED) then list[#list + 1] = value end
end

-- A bounded copy of a returned value (plain values, small tables).
local function boundedCopy(value, depth)
    if type(value) ~= "table" then return plain(value) end
    if depth >= MAX_COPY_DEPTH then return nil end
    local out, count = {}, 0
    for key, item in pairs(value) do
        if count >= MAX_COPY_ENTRIES then break end
        if plain(key) ~= nil then
            local copied = boundedCopy(item, depth + 1)
            if copied ~= nil then
                out[key] = copied
                count = count + 1
            end
        end
    end
    return out
end

local function isReader(name)
    for _, pattern in ipairs(READ_PREFIXES) do
        if name:find(pattern) then return true end
    end
    return false
end

-- Up to 3 return values of an argument-free reader, copied into discovery.readings.
local function readFunction(discovery, path, fn)
    local result = { pcall(fn) }
    if result[1] and #result > 1 then
        local values = {}
        for i = 2, math.min(#result, 4) do values[#values + 1] = boundedCopy(result[i], 0) end
        discovery.readings[path] = values
    end
end

-- Function names of a namespace, and the results of the argument-free readers (Get/Is/Has/Are only).
local function readNamespace(discovery, namespaceName, namespace)
    local names = {}
    for key, fn in pairs(namespace) do
        if type(key) == "string" and type(fn) == "function" then names[#names + 1] = key end
    end
    table.sort(names)
    discovery.namespaces[namespaceName] = names
    for _, name in ipairs(names) do
        if isReader(name) then readFunction(discovery, namespaceName .. "." .. name, namespace[name]) end
    end
end

-- Name, visibility and the plain fields of a frame (e.g. a configID or treeID set by the UI).
local function frameInfo(name, frame, extraConfigIDs)
    local info = { name = name, fields = {} }
    local okShown, shown = pcall(frame.IsShown, frame)
    if okShown then info.shown = shown and true or false end
    local count, children = 0, {}
    for key, value in pairs(frame) do
        if type(key) == "string" and plain(value) ~= nil and count < MAX_COPY_ENTRIES then
            info.fields[key] = value
            count = count + 1
            if key == "configID" and type(value) == "number" then extraConfigIDs[#extraConfigIDs + 1] = value end
        elseif type(key) == "string" and type(value) == "table" then
            append(children, key, MAX_COPY_ENTRIES)
        end
    end
    table.sort(children)
    if #children > 0 then info.children = children end
    return info
end

local function isFrame(value)
    return type(value) == "table" and type(value.GetObjectType) == "function"
end

local function addEnums(discovery)
    if type(Enum) ~= "table" then return end
    for enumName, values in pairs(Enum) do
        if type(enumName) == "string" and type(values) == "table" then
            local whole = mentionsLegacy(enumName) or enumName == "TraitConfigType" or enumName == "TraitSystemID"
            for key, value in pairs(values) do
                if (whole or mentionsLegacy(key)) and plain(value) ~= nil then
                    append(discovery.enums, "Enum." .. enumName .. "." .. tostring(key) .. "=" .. tostring(value))
                end
            end
        end
    end
    table.sort(discovery.enums)
end

-- Names worth reading, from one scan of Enum and _G.
local function scanNames()
    local names = { enums = {}, namespaces = {}, globals = {}, frames = {}, members = {} }
    addEnums(names)
    for key, value in pairs(_G) do
        if type(key) == "string" then
            if key:find("^C_") and type(value) == "table" and (mentionsLegacy(key) or mentionsChallenge(key)) then
                names.namespaces[#names.namespaces + 1] = key
            elseif key:find("^C_") and type(value) == "table" then
                -- The real client has no C_*Legacy* namespace (2026-11-22), so functions named for Legacy or
                -- challenges inside any namespace are listed too ("C_X.Name").
                for fnName, fn in pairs(value) do
                    if type(fn) == "function" and (mentionsLegacy(fnName) or mentionsChallenge(fnName)) then
                        append(names.members, key .. "." .. fnName)
                    end
                end
            elseif mentionsLegacy(key) then
                if type(value) == "function" then
                    append(names.globals, key)
                elseif isFrame(value) then
                    append(names.frames, key)
                end
            end
        end
    end
    table.sort(names.namespaces)
    table.sort(names.members)
    table.sort(names.globals)
    table.sort(names.frames)
    return names
end

local cachedNames -- scanNames() result for this session; nil = scan on the next dump

-- The Legacy window is a load-on-demand addon: its frames and functions appear only after it loads.
ns.Events.on("ADDON_LOADED", function(addonName)
    if mentionsLegacy(addonName) then cachedNames = nil end
end)

local function discover(extraConfigIDs)
    cachedNames = cachedNames or scanNames()
    local discovery = { enums = cachedNames.enums, namespaces = {}, readings = {}, globals = cachedNames.globals, frames = {},
        members = cachedNames.members }
    for _, name in ipairs(cachedNames.namespaces) do
        if type(_G[name]) == "table" then readNamespace(discovery, name, _G[name]) end
    end
    for _, path in ipairs(cachedNames.members) do
        local namespaceName, fnName = path:match("^([^.]+)%.(.+)$")
        local namespace = _G[namespaceName]
        if isReader(fnName) and type(namespace) == "table" and type(namespace[fnName]) == "function" then
            readFunction(discovery, path, namespace[fnName])
        end
    end
    for _, name in ipairs(cachedNames.frames) do
        if isFrame(_G[name]) then
            discovery.frames[#discovery.frames + 1] = frameInfo(name, _G[name], extraConfigIDs)
        end
    end
    return discovery
end

-- Extra config IDs found on Legacy frames are appended to extraConfigIDs.
LegacyDiscovery.discover = discover
