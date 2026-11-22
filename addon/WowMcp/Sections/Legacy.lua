-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section legacy: the Forever Legacy system (account-wide Legacy Points, per-character perk trees
-- Professions / Adventure / Resourcefulness with an "Apply Changes" button, challenges and rewards).
--
-- No public documentation names the API behind it, so the section does not hard-code one:
--   * configs: every C_Traits config that is not the class talents (C_Traits.GetConfigsByType for each
--     Enum.TraitConfigType, plus a configID found on a Legacy frame), read like the talents (TraitTree);
--   * challenges: achievements in categories whose title contains "Legacy" (English client text);
--   * discovery: Enum entries, C_* namespaces (named "Legacy" or "Challenge"), functions and frames whose name
--     contains "Legacy", and the results of the namespaces' argument-free Get/Is/Has/Are functions, so the next
--     version can use the real API. The scan of _G is costly (78 ms of a 112 ms real dump), so the names are found
--     once per session and again only after the client loads an addon with "Legacy" in its name (the Legacy window
--     is load-on-demand); the readings and frame fields are fresh on every dump.
-- Real client (2026-11-22): the perks are a Generic config with trees 1187, 1188, 1189 and no named groups.
-- Everything is a read. Nothing here stages, learns, refunds or applies a perk.

local _, ns = ...
local Compat = ns.Compat
local plain = Compat.plain
local TraitTree = ns.TraitTree

local MAX_DISCOVERED = 60       -- entries per discovery list
local MAX_COPY_ENTRIES = 40     -- entries per table copied from a reading
local MAX_COPY_DEPTH = 3        -- nesting of a copied reading
local MAX_ACHIEVEMENTS = 150
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

---------------------------------------------------------------------------
-- Perk trees (C_Traits configs other than the class talents)
---------------------------------------------------------------------------

-- Enum.TraitConfigType: value -> name (empty when the client has no such enum).
local function configTypeNames()
    local names = {}
    local enum = Enum and Enum.TraitConfigType
    if type(enum) == "table" then
        for key, value in pairs(enum) do
            if type(key) == "string" and type(value) == "number" then names[value] = key end
        end
    end
    return names
end

-- Config types worth asking about: all known ones except Invalid and Combat (class talents);
-- without the enum, the small numbers the enum uses in the modern client.
local function candidateTypes(names)
    local types = {}
    if next(names) then
        for value, name in pairs(names) do
            if name ~= "Invalid" and name ~= "Combat" then types[#types + 1] = value end
        end
    else
        for value = 0, 15 do types[#types + 1] = value end
    end
    table.sort(types)
    return types
end

-- The account trait currency behind a tree currency (name, total earned), when the client links one.
local function addAccountCurrency(currency)
    local id = currency.currencyID
    if not (id and C_Traits.GetTraitCurrencyInfo) then return end
    local ok, _, _, currencyTypesID = pcall(C_Traits.GetTraitCurrencyInfo, id)
    currencyTypesID = ok and plain(currencyTypesID)
    if currencyTypesID and C_CurrencyInfo and C_CurrencyInfo.GetCurrencyInfo then
        local okInfo, info = pcall(C_CurrencyInfo.GetCurrencyInfo, currencyTypesID)
        if okInfo and type(info) == "table" then
            currency.currencyTypesID = currencyTypesID
            currency.name = plain(info.name)
            currency.quantity = plain(info.quantity)
            currency.totalEarned = plain(info.totalEarned)
            currency.maxQuantity = plain(info.maxQuantity)
        end
    end
    if C_Traits.GetMaxAvailableTraitCurrency then
        local okMax, maxAvailable = pcall(C_Traits.GetMaxAvailableTraitCurrency, id)
        if okMax then currency.maxAvailable = plain(maxAvailable) end
    end
end

-- Locks from node conditions (e.g. points to spend in the tree first).
local function addConditions(entry, configID, node)
    if not C_Traits.GetConditionInfo then return end
    for _, conditionID in ipairs(node.conditionIDs or {}) do
        local ok, condition = pcall(C_Traits.GetConditionInfo, configID, conditionID)
        if ok and type(condition) == "table" and condition.isMet == false then
            entry.locked = true
            local required = plain(condition.spentAmountRequired)
            if required and required > (entry.requiresSpent or 0) then entry.requiresSpent = required end
        end
    end
end

-- Reads one node; also remembers its arrows (visibleEdges) to fill "requires" later.
local function readNode(config, configID, nodeID, node, groupNames, edges)
    local treeName = TraitTree.tabOf(node, groupNames)
    local entry = TraitTree.nodeEntry(configID, nodeID, node, treeName, true)
    entry.nodeID = plain(nodeID)
    if node.meetsEdgeRequirements ~= nil then entry.prereqMet = node.meetsEdgeRequirements and true or false end
    TraitTree.addDescription(config, entry, TraitTree.entryOf(node), math.max(entry.rank, 1))
    addConditions(entry, configID, node)
    for _, edge in ipairs(node.visibleEdges or {}) do
        local target = plain(edge.targetNode)
        if target then
            edges[target] = edges[target] or {}
            table.insert(edges[target], nodeID)
        end
    end
    return entry
end

-- Fills "requires" (names of the nodes whose arrow points at a node).
local function addRequirements(nodes, edges)
    local names = {}
    for _, entry in ipairs(nodes) do
        if entry.nodeID then names[entry.nodeID] = entry.name end
    end
    for _, entry in ipairs(nodes) do
        for _, source in ipairs(edges[entry.nodeID] or {}) do
            entry.requires = entry.requires or {}
            table.insert(entry.requires, names[source] or ("node " .. tostring(source)))
        end
    end
end

-- Plain fields of C_Traits.GetTreeInfo (a tree without named groups may carry its title there).
local function treeInfo(configID, treeID)
    if not C_Traits.GetTreeInfo then return nil end
    local ok, info = pcall(C_Traits.GetTreeInfo, configID, treeID)
    if not (ok and type(info) == "table") then return nil end
    local out = {}
    for key, value in pairs(info) do
        if type(key) == "string" and plain(value) ~= nil then out[key] = value end
    end
    return Compat.pick(next(out) ~= nil, out, nil)
end

local function readTree(config, configID, treeID, edges)
    local groupNames, groups = TraitTree.tabs(treeID)
    for _, currency in ipairs(TraitTree.currencies(configID, treeID)) do
        currency.treeID = plain(treeID)
        addAccountCurrency(currency)
        config.currencies[#config.currencies + 1] = currency
    end
    local fallbackName = next(groupNames) == nil and ("tree " .. tostring(treeID)) or nil
    local points, count = {}, 0
    for _, nodeID in ipairs(C_Traits.GetTreeNodes(treeID) or {}) do
        local node = C_Traits.GetNodeInfo(configID, nodeID)
        if node and node.isVisible ~= false and (node.maxRanks or 0) > 0 then
            local entry = readNode(config, configID, nodeID, node, groupNames, edges)
            entry.tree = entry.tree or fallbackName
            if entry.tree then points[entry.tree] = (points[entry.tree] or 0) + entry.rank end
            config.nodes[#config.nodes + 1] = entry
            count = count + 1
        end
    end
    if fallbackName then
        local tree = { name = fallbackName, treeID = plain(treeID), points = points[fallbackName] or 0, nodes = count }
        tree.info = treeInfo(configID, treeID)
        config.trees[#config.trees + 1] = tree
    end
    for _, group in ipairs(groups) do
        config.trees[#config.trees + 1] = {
            name = group.displayName, treeID = plain(treeID), points = points[group.displayName] or 0,
        }
    end
end

local function readConfig(configID, typeName)
    local info = C_Traits.GetConfigInfo(configID)
    if type(info) ~= "table" then return nil end
    local config = {
        configID = plain(configID), type = plain(info.type), typeName = typeName, name = plain(info.name),
        trees = {}, nodes = {}, currencies = {},
    }
    local edges = {}
    for _, treeID in ipairs(info.treeIDs or {}) do
        readTree(config, configID, treeID, edges)
    end
    addRequirements(config.nodes, edges)
    ns.TalentGrid.place(config.nodes)
    return config
end

-- Config IDs by type, without the active class talent config.
local function findConfigs(typeNames, extraConfigIDs)
    local talentConfig = C_ClassTalents and C_ClassTalents.GetActiveConfigID and C_ClassTalents.GetActiveConfigID()
    local found, seen = {}, {}
    local function add(configID, typeName)
        configID = plain(configID)
        if configID and configID ~= talentConfig and not seen[configID] then
            seen[configID] = true
            found[#found + 1] = { id = configID, typeName = typeName }
        end
    end
    if C_Traits.GetConfigsByType then
        for _, configType in ipairs(candidateTypes(typeNames)) do
            local ok, ids = pcall(C_Traits.GetConfigsByType, configType)
            if ok and type(ids) == "table" then
                for _, configID in ipairs(ids) do add(configID, typeNames[configType] or tostring(configType)) end
            end
        end
    end
    for _, configID in ipairs(extraConfigIDs) do add(configID, "frame") end
    return found
end

local function readConfigs(legacy, typeNames, extraConfigIDs)
    if not (C_Traits and C_Traits.GetConfigInfo and C_Traits.GetTreeNodes and C_Traits.GetNodeInfo) then return end
    for _, found in ipairs(findConfigs(typeNames, extraConfigIDs)) do
        local ok, config = pcall(readConfig, found.id, found.typeName)
        if ok and config then
            legacy.configs[#legacy.configs + 1] = config
        elseif not ok then
            legacy.configErrors = legacy.configErrors or {}
            append(legacy.configErrors, tostring(found.id) .. ": " .. tostring(config), 10)
        end
    end
end

---------------------------------------------------------------------------
-- Challenges (achievements in a "Legacy" category)
---------------------------------------------------------------------------

local function legacyCategories()
    local titles, parents, picked = {}, {}, {}
    for _, categoryID in ipairs(GetCategoryList() or {}) do
        local title, parentID = GetCategoryInfo(categoryID)
        titles[categoryID], parents[categoryID] = plain(title), plain(parentID)
        if mentionsLegacy(plain(title)) then picked[categoryID] = true end
    end
    local list = {}
    for categoryID in pairs(titles) do
        if picked[categoryID] or picked[parents[categoryID]] then
            list[#list + 1] = { id = categoryID, title = titles[categoryID] }
        end
    end
    table.sort(list, function(a, b) return a.id < b.id end)
    return list
end

local function readChallenges()
    if not (GetCategoryList and GetCategoryInfo and GetCategoryNumAchievements and GetAchievementInfo) then return nil end
    local challenges = { matchedBy = "achievement category title contains Legacy", list = {} }
    for _, category in ipairs(legacyCategories()) do
        for index = 1, GetCategoryNumAchievements(category.id, true) or 0 do
            local id, name, points, completed, _, _, _, description, _, _, rewardText, _, earnedByMe =
                GetAchievementInfo(category.id, index)
            if id and #challenges.list < MAX_ACHIEVEMENTS then
                challenges.list[#challenges.list + 1] = {
                    id = plain(id), category = category.title, name = plain(name), points = plain(points),
                    completed = completed and true or false, earnedByMe = Compat.pick(earnedByMe == nil, nil, earnedByMe and true or false),
                    description = plain(description), reward = Compat.pick(plain(rewardText) == "", nil, plain(rewardText)),
                }
            end
        end
    end
    return challenges
end

---------------------------------------------------------------------------
-- Discovery: what this client calls the Legacy API
---------------------------------------------------------------------------

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

---------------------------------------------------------------------------

local function collectLegacy()
    local legacy = { configs = {} }
    local extraConfigIDs = {}
    legacy.discovery = discover(extraConfigIDs)
    readConfigs(legacy, configTypeNames(), extraConfigIDs)
    local ok, challenges = pcall(readChallenges)
    if ok then
        legacy.challenges = challenges
    else
        legacy.challengesError = tostring(challenges)
    end
    legacy.source = Compat.pick(#legacy.configs > 0, "traits", "none")
    return legacy
end

ns.Snapshot.addSection("legacy", collectLegacy)
