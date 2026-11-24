-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section eventLog: timestamped events recorded while playing (level-ups, gold, quests, zones, equipment),
-- oldest first. The game writes SavedVariables only on /reload or logout, so the log tells the server when
-- things happened in between. Carried over every save and login (the addon never assumes the server read
-- a dump); bounded by count and by age. Only records what the client reports.

local _, ns = ...
local plain, firstValue = ns.Compat.plain, ns.Compat.firstValue

local EventLog = {}
ns.EventLog = EventLog

EventLog.MAX_ENTRIES = 300
EventLog.MAX_AGE = 14 * 24 * 3600
EventLog.MONEY_THROTTLE = 300

local function entries()
    local current = ns.Snapshot.current()
    if type(current.eventLog) ~= "table" then current.eventLog = {} end
    if type(current.eventLog.entries) ~= "table" then current.eventLog.entries = {} end
    return current.eventLog.entries
end

local function lastOfKind(list, kind)
    for i = #list, 1, -1 do
        if list[i].kind == kind then return list[i], i end
    end
    return nil
end

local function append(entry)
    local list = entries()
    entry.at = time()
    list[#list + 1] = entry
    while #list > EventLog.MAX_ENTRIES do table.remove(list, 1) end
end

local function dropOld()
    local list, kept, now = entries(), {}, time()
    for _, entry in ipairs(list) do
        if type(entry) == "table" and type(entry.at) == "number" and now - entry.at <= EventLog.MAX_AGE then
            kept[#kept + 1] = entry
        end
    end
    ns.Snapshot.current().eventLog.entries = kept
end

local function onLevelUp(level)
    level = plain(level)
    if type(level) == "number" then append({ kind = "level", level = level }) end
end

local function onMoney()
    local copper = firstValue({ "GetMoney" })
    if type(copper) ~= "number" then return end
    local list = entries()
    local last = list[#list]
    if last and last.kind == "money" and time() - last.at < EventLog.MONEY_THROTTLE then
        last.copper = copper
        return
    end
    append({ kind = "money", copper = copper })
end

-- Classic passes (questLogIndex, questID), mainline only (questID): the quest ID is the last number.
local function lastNumber(...)
    for i = select("#", ...), 1, -1 do
        local value = plain((select(i, ...)))
        if type(value) == "number" then return value end
    end
    return nil
end

local function onQuestAccepted(...)
    local questID = lastNumber(...)
    if not questID then return end
    local title = C_QuestLog and C_QuestLog.GetTitleForQuestID and plain(C_QuestLog.GetTitleForQuestID(questID))
    append({ kind = "questAccepted", questID = questID, title = (type(title) == "string" and title ~= "") and title or nil })
end

-- QUEST_TURNED_IN(questID, xpReward, moneyReward)
local function onQuestTurnedIn(questID, xp, copper)
    questID = plain(questID)
    if type(questID) ~= "number" then return end
    append({ kind = "questTurnedIn", questID = questID, xp = plain(xp), copper = plain(copper) })
end

local function onZoneChanged()
    local zone = firstValue({ "GetRealZoneText" })
    if type(zone) ~= "string" or zone == "" then return end
    local previous = lastOfKind(entries(), "zone")
    if previous and previous.zone == zone then return end
    append({ kind = "zone", zone = zone })
end

-- PLAYER_EQUIPMENT_CHANGED(slot, hasCurrent)
local function onEquipmentChanged(slot)
    slot = plain(slot)
    if type(slot) ~= "number" then return end
    append({ kind = "equipment", slot = slot, itemID = firstValue({ "GetInventoryItemID" }, "player", slot) })
end

local function guarded(handler)
    return ns.Snapshot.whenEnabled("eventLog", function(...) pcall(handler, ...) end)
end

ns.Snapshot.addWindowSection("eventLog", nil, nil, { quiet = true })
ns.Events.on("PLAYER_LOGIN", guarded(dropOld))
ns.Events.on("PLAYER_LEVEL_UP", guarded(onLevelUp))
ns.Events.on("PLAYER_MONEY", guarded(onMoney))
ns.Events.on("QUEST_ACCEPTED", guarded(onQuestAccepted))
ns.Events.on("QUEST_TURNED_IN", guarded(onQuestTurnedIn))
ns.Events.on("ZONE_CHANGED_NEW_AREA", guarded(onZoneChanged))
ns.Events.on("PLAYER_EQUIPMENT_CHANGED", guarded(onEquipmentChanged))
