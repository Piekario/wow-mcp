-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section talents: spent points (points) and full trees with rank-0 nodes (nodes), so that it is possible to
-- advise where to spend further points. Data comes solely from the client API,
-- because trees in Forever differ from the classic ones.

local _, ns = ...
local Compat = ns.Compat
local plain, firstValue = Compat.plain, Compat.firstValue
local TraitTree = ns.TraitTree

local function spentPoint(tree, name, rank, maxRank)
    return { tree = tree, name = name, rank = rank, maxRank = maxRank }
end

---------------------------------------------------------------------------
-- Classic trees (GetTalentInfo)
---------------------------------------------------------------------------

local function classicTabName(tab)
    if not GetTalentTabInfo then return nil end
    local a, b = GetTalentTabInfo(tab)
    -- The order of returned values differs between versions: the name is the first string.
    return (type(a) == "string" and a) or (type(b) == "string" and b) or tostring(tab)
end

-- Adds the talents of one tab; returns the number of points spent in it.
local function addClassicTab(talents, tab)
    local tabName = classicTabName(tab)
    local inTree = 0
    for index = 1, GetNumTalents(tab) or 0 do
        -- name, icon, tier, column, rank, maxRank, isExceptional, meetsPrereq
        local name, _, tier, column, rank, maxRank, _, meetsPrereq = GetTalentInfo(tab, index)
        if name then
            rank = plain(rank) or 0
            local node = {
                tree = tabName, name = name, rank = rank, maxRank = plain(maxRank),
                row = plain(tier), column = plain(column),
            }
            if meetsPrereq ~= nil then node.prereqMet = meetsPrereq and true or false end
            talents.nodes[#talents.nodes + 1] = node
            if rank > 0 then
                inTree = inTree + rank
                talents.points[#talents.points + 1] = spentPoint(tabName, name, rank, plain(maxRank))
            end
        end
    end
    talents.trees[#talents.trees + 1] = { name = tabName, points = inTree }
    return inTree
end

local function addClassicTalents(talents)
    talents.source = "classic"
    talents.trees = {}
    local spent = 0
    for tab = 1, GetNumTalentTabs() or 0 do
        spent = spent + addClassicTab(talents, tab)
    end
    talents.spent = spent
    talents.unspent = firstValue({ "UnitCharacterPoints", "GetUnspentTalentPoints" }, "player")
end

---------------------------------------------------------------------------
-- New API (C_Traits). In Forever the classic trees sit on the traits system:
-- one config with one tree per class, and three tabs (e.g. Beast
-- Mastery, Marksmanship, Survival) are node groups with their own name.
---------------------------------------------------------------------------

-- Sum of spent and free points in the whole tree.
local function addTreeCurrency(talents, configID, treeID)
    local currency = TraitTree.currencies(configID, treeID)[1]
    if currency then
        talents.spent = (talents.spent or 0) + (currency.spent or 0)
        talents.unspent = (talents.unspent or 0) + (currency.available or 0)
    end
end

local function addTraitTree(talents, configID, treeID)
    local groupNames, groups = TraitTree.tabs(treeID)
    local pointsByTree = {}
    addTreeCurrency(talents, configID, treeID)

    for _, nodeID in ipairs(C_Traits.GetTreeNodes(treeID) or {}) do
        local node = C_Traits.GetNodeInfo(configID, nodeID)
        -- Hidden nodes (e.g. from another class in the same tree) are skipped.
        if node and node.isVisible ~= false and (node.maxRanks or 0) > 0 then
            local rank = TraitTree.rankOf(node)
            local treeName = TraitTree.tabOf(node, groupNames)
            if treeName and rank > 0 then
                pointsByTree[treeName] = (pointsByTree[treeName] or 0) + rank
            end

            -- Nodes without a named tab (when tabs are known) are usually nodes of another class.
            local keep = treeName ~= nil or next(groupNames) == nil or rank > 0
            local entry = TraitTree.nodeEntry(configID, nodeID, node, treeName, keep)
            if keep then
                TraitTree.addDescription(talents, entry, nil, rank)
                talents.nodes[#talents.nodes + 1] = entry
            end
            if rank > 0 then
                talents.points[#talents.points + 1] = spentPoint(treeName, entry.name, rank, plain(node.maxRanks))
            end
        end
    end

    for _, group in ipairs(groups) do
        talents.trees[#talents.trees + 1] = {
            name = group.displayName, points = pointsByTree[group.displayName] or 0,
        }
    end
end

local function addTraitTalents(talents)
    talents.source = "traits"
    talents.trees = {}
    local configID = C_ClassTalents.GetActiveConfigID()
    if not configID then return end
    local config = C_Traits.GetConfigInfo(configID)
    for _, treeID in ipairs((config and config.treeIDs) or {}) do
        addTraitTree(talents, configID, treeID)
    end
    ns.TalentGrid.place(talents.nodes)
end

local function collectTalents()
    local talents = { points = {}, nodes = {} }
    -- Classic trees are the simplest source, if the client has them.
    if GetNumTalentTabs and GetNumTalents and GetTalentInfo then
        addClassicTalents(talents)
    elseif C_ClassTalents and C_ClassTalents.GetActiveConfigID and C_Traits and C_Traits.GetConfigInfo then
        addTraitTalents(talents)
    end
    return talents
end

ns.Snapshot.addSection("talents", collectTalents)
