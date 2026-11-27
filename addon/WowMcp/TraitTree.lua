-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Reading trait trees (C_Traits): shared by the class talents (Sections/Talents) and the Legacy perks
-- (Sections/Legacy). Only reads; never stages, purchases or commits anything.

local _, ns = ...
local Compat = ns.Compat
local plain, firstValue, callFirst = Compat.plain, Compat.firstValue, Compat.callFirst
local stripMarkup, tooltipFor = Compat.stripMarkup, Compat.tooltipFor

local TraitTree = {}
ns.TraitTree = TraitTree

-- Name and spell of a tree entry.
function TraitTree.entryInfo(configID, entryID)
    if not entryID then return nil end
    local entry = C_Traits.GetEntryInfo(configID, entryID)
    local def = entry and entry.definitionID and C_Traits.GetDefinitionInfo(entry.definitionID)
    if not def then return nil end
    local spellID = plain(def.spellID) or plain(def.overriddenSpellID)
    local name = plain(def.overrideName)
    if not name and spellID then
        name = firstValue({ "C_Spell.GetSpellName", "GetSpellInfo" }, spellID)
    end
    return name, spellID
end

-- Description from the spell API. In a real client it is sometimes empty until the client loads the spell data.
local function apiDescription(spellID)
    local text = firstValue({ "C_Spell.GetSpellDescription", "GetSpellDescription" }, spellID)
    if type(text) ~= "string" or not text:find("%S") then return nil end
    return stripMarkup(text)
end

-- Description of the entry itself (C_Traits.GetTraitDescription), for entries whose spell has no text.
local function traitDescription(entryID, rank)
    if not (entryID and C_Traits.GetTraitDescription) then return nil end
    local text = plain(C_Traits.GetTraitDescription(entryID, rank))
    if type(text) ~= "string" or not text:find("%S") then return nil end
    return stripMarkup(text)
end

-- Description and where it came from ("api", "trait" or "tooltip"); nil when the client does not know it yet.
-- Talents (no entryID) keep the old order api -> tooltip. Trait entries (Legacy perks) try the entry text before the
-- tooltip, and a tooltip with a single line under the name is not a description: before the spell data loads the
-- real client shows only the spell type there (seen in the first real Legacy dump, 2026-11-22).
local function describe(spellID, entryID, rank)
    if spellID then
        local text = apiDescription(spellID)
        if text then return text, "api" end
    end
    local text = traitDescription(entryID, rank)
    if text then return text, "trait" end
    if spellID then
        local lines = tooltipFor("GetSpellByID", spellID)
        local minimum = entryID and 3 or 2
        if lines and #lines >= minimum then return table.concat(lines, "\n", 2), "tooltip" end
    end
    return nil
end

-- Description of a spell by ID and where it came from; nil when the client does not know it yet.
function TraitTree.spellDescription(spellID)
    local ok, text, source = pcall(describe, spellID)
    if ok then return text, source end
    return nil
end

-- Adds a description to a node entry. No description: a request for spell data (it only loads data into client
-- memory, not a game action) and the owner's missingDescriptions counter, so the server knows another dump will help.
function TraitTree.addDescription(owner, entry, entryID, rank)
    local ok, text, source = pcall(describe, entry.spellID, entryID, rank)
    if ok and text then
        entry.description = text
        if source ~= "api" then entry.descriptionSource = source end
        return
    end
    if entry.spellID then
        pcall(callFirst, { "C_Spell.RequestLoadSpellData" }, entry.spellID)
    end
    if entry.spellID or (entryID and C_Traits.GetTraitDescription) then
        owner.missingDescriptions = (owner.missingDescriptions or 0) + 1
    end
end

-- Tree tabs: groupID -> displayName and the list of named groups in display order.
function TraitTree.tabs(treeID)
    local groupNames, groups = {}, {}
    if C_Traits.GetGroupDisplayInfoByTreeID then
        for _, group in ipairs(C_Traits.GetGroupDisplayInfoByTreeID(treeID) or {}) do
            if group.groupID and group.displayName then
                groupNames[group.groupID] = group.displayName
                groups[#groups + 1] = group
            end
        end
        table.sort(groups, function(a, b) return (a.orderIndex or 0) < (b.orderIndex or 0) end)
    end
    return groupNames, groups
end

-- A node's tab is the first of its groups that has a display name.
function TraitTree.tabOf(node, groupNames)
    for _, groupID in ipairs(node.groupIDs or {}) do
        if groupNames[groupID] then return groupNames[groupID] end
    end
    return nil
end

-- Option names of a node with a choice of one of several.
local function choices(configID, entryIDs)
    local names = {}
    for _, choiceID in ipairs(entryIDs) do
        local ok, choiceName = pcall(TraitTree.entryInfo, configID, choiceID)
        if ok and choiceName then names[#names + 1] = choiceName end
    end
    return names
end

-- The entry a node shows: the chosen one, else the first.
function TraitTree.entryOf(node)
    return (node.activeEntry and node.activeEntry.entryID) or (node.entryIDs and node.entryIDs[1])
end

-- Current rank of a node.
function TraitTree.rankOf(node)
    return plain(node.activeRank or node.currentRank) or 0
end

-- Node entry: tab, name, rank, spell, raw position (row and column come from TalentGrid.place) and the game's flags.
-- named = false skips the name lookup (nodes that will be dropped anyway).
function TraitTree.nodeEntry(configID, nodeID, node, treeName, named)
    local entryID = named and TraitTree.entryOf(node)
    local okName, name, spellID = pcall(TraitTree.entryInfo, configID, entryID)
    if not okName then name, spellID = nil, nil end
    name = name or ("node " .. tostring(nodeID))

    local entry = {
        tree = treeName, name = name, rank = TraitTree.rankOf(node), maxRank = plain(node.maxRanks),
        spellID = spellID,
        posX = plain(node.posX), posY = plain(node.posY),
    }
    if node.canPurchaseRank ~= nil then entry.canPurchase = node.canPurchaseRank and true or false end
    if node.isAvailable ~= nil then entry.available = node.isAvailable and true or false end
    if node.entryIDs and #node.entryIDs > 1 then
        entry.choices = choices(configID, node.entryIDs)
    end
    return entry
end

-- Spent and available points of a tree, one entry per trait currency.
function TraitTree.currencies(configID, treeID)
    local out = {}
    if not C_Traits.GetTreeCurrencyInfo then return out end
    for _, currency in ipairs(C_Traits.GetTreeCurrencyInfo(configID, treeID, false) or {}) do
        out[#out + 1] = {
            currencyID = plain(currency.traitCurrencyID),
            available = plain(currency.quantity),
            spent = plain(currency.spent),
            max = plain(currency.maxQuantity),
        }
    end
    return out
end
