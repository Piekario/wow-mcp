-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section inspect (from the window): equipment of players inspected via the Inspect window, name -> result.
-- We save only when the player clicked Inspect themselves (the INSPECT_READY event);
-- the addon sends no inspection requests.

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue
local round1, Items = ns.round1, ns.Items

local INSPECT_MAX = 30
local INSPECT_UNITS = { "target", "mouseover", "focus" }
local SHIRT_SLOT, TABARD_SLOT = 4, 19

local function unitMatches(unit, guid)
    return not guid or firstValue({ "UnitGUID" }, unit) == guid
end

-- Units under which the inspected player may be found when the Inspect window does not give its own.
local function candidateUnits()
    local candidates = {}
    for _, u in ipairs(INSPECT_UNITS) do candidates[#candidates + 1] = u end
    for i = 1, 4 do candidates[#candidates + 1] = "party" .. i end
    for i = 1, 40 do candidates[#candidates + 1] = "raid" .. i end
    return candidates
end

local function inspectUnitFor(guid)
    if InspectFrame and type(InspectFrame.unit) == "string" then
        local u = InspectFrame.unit
        if unitMatches(u, guid) then return u end
    end
    for _, u in ipairs(candidateUnits()) do
        if UnitExists and UnitExists(u) and unitMatches(u, guid) then return u end
    end
    return nil
end

local function inspectedItem(unit, slot)
    local link = firstValue({ "GetInventoryItemLink" }, unit, slot)
    if not link then return nil end
    local item = { slot = Items.EQUIPMENT_SLOTS[slot] or tostring(slot), id = Items.idFromLink(link), name = Items.nameFromLink(link) }
    local info = callFirst({ "C_Item.GetItemInfo", "GetItemInfo" }, link)
    if info then
        item.quality = plain(info[3])
        item.itemLevel = plain(info[4])
    end
    return item
end

-- Adds the equipment; returns the average item level or nil.
local function addInspectedEquipment(out, unit)
    local levels, sum = 0, 0
    for slot = 1, 19 do
        -- Shirt and tabard do not count toward item level.
        if slot ~= SHIRT_SLOT and slot ~= TABARD_SLOT then
            local item = inspectedItem(unit, slot)
            if item then
                if type(item.itemLevel) == "number" then
                    levels, sum = levels + 1, sum + item.itemLevel
                end
                out.equipment[#out.equipment + 1] = item
            end
        end
    end
    if levels > 0 then return round1(sum / levels) end
    return nil
end

local function collectInspect(guid)
    local unit = inspectUnitFor(guid)
    if not unit then return nil end
    local out = ns.Group.unitIdentity(unit)
    if not out then return nil end
    out.capturedAt = time()
    out.equipment = {}
    local guild = callFirst({ "GetGuildInfo" }, unit)
    if guild then out.guild = plain(guild[1]) end
    out.averageItemLevel = addInspectedEquipment(out, unit)
    -- The level supplied by the client is more accurate than the average from the links.
    local ilvl = firstValue({ "C_PaperDollInfo.GetInspectItemLevel" }, unit)
    if type(ilvl) == "number" and ilvl > 0 then out.averageItemLevel = round1(ilvl) end
    if #out.equipment == 0 then return nil end
    return out
end

local function captureInspect(guid)
    local ok, result = pcall(collectInspect, guid)
    if not ok or not result then return end
    local current = ns.Snapshot.current()
    if type(current.inspect) ~= "table" then current.inspect = {} end
    current.inspect[result.name] = result
    -- The longest-ago inspected fall off when there are too many.
    ns.Snapshot.keepNewest(current.inspect, INSPECT_MAX)
    ns.Snapshot.windowCaptured("inspect")
end

ns.Snapshot.addWindowSection("inspect")
ns.Events.on("INSPECT_READY", ns.Snapshot.whenEnabled("inspect", captureInspect))
