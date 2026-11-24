-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section dungeonLoot: what the bosses of a dungeon drop, read from the Encounter Journal only when the player
-- types /mcp loot [dungeon]; never on a dump, because the read moves the journal window's selection (it is put
-- back). Without a name: the dungeon the character is in, otherwise the dungeons for the character's level (from
-- the saved dungeons section). Results pass through later dumps, newest first and bounded. Item details come from
-- Items.describe; items the client has not loaded yet are marked pending and read again a few times as data arrives.
-- Every /mcp loot leaves lastAttempt in the section (outcome, what the journal offered), so a dump says what happened.

local _, ns = ...
local plain, chat = ns.Compat.plain, ns.chat
local EJ, Snapshot = ns.EncounterJournal, ns.Snapshot

local DungeonLoot = {}
ns.DungeonLoot = DungeonLoot

local SECTION = "dungeonLoot"
local MAX_SAVED = 8
local MAX_ITEMS = 40
local MAX_NAMES = 40
local MAX_PER_COMMAND = 3
local MAX_RETRIES = 5
local RETRY_SECONDS = 1

local retry = { left = 0, scheduled = false }

local function normalName(name)
    return (tostring(name):lower():gsub("^%s+", ""):gsub("%s+$", ""):gsub("^the%s+", ""))
end

local function positive(...)
    for i = 1, select("#", ...) do
        local value = plain((select(i, ...)))
        if type(value) == "number" and value > 0 then return value end
    end
    return nil
end

local function listOf(value)
    if type(value) == "table" then return value end
    return {}
end

-- Level ranges of the group finder from the last saved dump: { name, mapID, min, max }.
local function savedLevelRanges()
    local saved = ns.Store.savedCurrentCharacter()
    local dungeons = type(saved) == "table" and saved.dungeons
    local ranges = {}
    if type(dungeons) ~= "table" then return ranges end
    for _, a in ipairs(listOf(dungeons.activities)) do
        ranges[#ranges + 1] = { name = a.name, min = positive(a.minLevelSuggestion, a.minLevel),
                                max = positive(a.maxLevelSuggestion, a.maxLevel) }
    end
    for _, d in ipairs(listOf(dungeons.lfg)) do
        ranges[#ranges + 1] = { name = d.name, mapID = positive(d.mapID), min = positive(d.minRecLevel, d.recLevel, d.minLevel),
                                max = positive(d.maxRecLevel, d.maxLevel) }
    end
    return ranges
end

local function rangeFor(instance, ranges)
    for _, r in ipairs(ranges) do
        local sameMap = r.mapID ~= nil and r.mapID == instance.mapID
        if r.min and (sameMap or (type(r.name) == "string" and normalName(r.name) == normalName(instance.name))) then
            return r
        end
    end
    return nil
end

local function byName(instances, query)
    local wanted = normalName(query)
    local exact, partial = {}, {}
    for _, instance in ipairs(instances) do
        local name = normalName(instance.name)
        if name == wanted then
            exact[#exact + 1] = instance
        elseif name:find(wanted, 1, true) then
            partial[#partial + 1] = instance
        end
    end
    if #exact > 0 then return exact end
    return partial
end

local function forLevel(instances, ranges)
    local level = plain(UnitLevel("player"))
    local out = {}
    if type(level) ~= "number" then return out end
    for _, instance in ipairs(instances) do
        local r = rangeFor(instance, ranges)
        if r and not instance.raid and level >= r.min - 1 and level <= (r.max or r.min) then out[#out + 1] = instance end
    end
    return out
end

-- Which journal instances to read for the player's query.
local function chooseTargets(query, instances, ranges)
    if query ~= "" then return byName(instances, query) end
    local current = EJ.currentInstanceID()
    for _, instance in ipairs(instances) do
        if instance.id == current then return { instance } end
    end
    return forLevel(instances, ranges)
end

-- Fills the item from the client's item data; pending marks an item whose data has not arrived yet.
local function describe(item, link)
    ns.Items.describe(item, link)
    item.pending = ns.Compat.pick(item.itemLevel == nil, true, nil)
end

local function buildEntry(instance, read, ranges)
    local bosses = {}
    local entry = { name = instance.name, journalInstanceID = instance.id, mapID = instance.mapID, raid = instance.raid,
                    capturedAt = time(), encounters = {}, items = {}, truncated = read.truncated }
    for _, encounter in ipairs(read.encounters) do
        entry.encounters[#entry.encounters + 1] = encounter.name
        if encounter.id then bosses[encounter.id] = encounter.name end
    end
    local range = rangeFor(instance, ranges)
    if range then entry.minLevel, entry.maxLevel = range.min, range.max end
    for _, row in ipairs(read.rows) do
        local item = { id = row.id, name = row.name, quality = row.quality, slotText = row.slotText,
                       armorType = row.armorType, encounter = bosses[row.encounterID], encounterID = row.encounterID }
        describe(item, row.link)
        entry.items[#entry.items + 1] = item
    end
    return entry
end

local function section()
    local current = Snapshot.current()
    if type(current[SECTION]) ~= "table" then current[SECTION] = {} end
    local out = current[SECTION]
    if type(out.instances) ~= "table" then out.instances = {} end
    return out
end

-- Newest first; a dungeon read again replaces its older entry, and the oldest fall off beyond MAX_SAVED.
local function remember(entry)
    local saved = section()
    local list = { entry }
    for _, old in ipairs(saved.instances) do
        if old.journalInstanceID ~= entry.journalInstanceID and #list < MAX_SAVED then list[#list + 1] = old end
    end
    saved.instances = list
    saved.capturedAt = entry.capturedAt
    saved.unavailable, saved.checkedAt = nil, nil
end

local function pendingItems()
    local saved = ns.Store.savedCurrentCharacter()
    local loot = type(saved) == "table" and saved[SECTION]
    local out = {}
    for _, instance in ipairs(type(loot) == "table" and listOf(loot.instances) or {}) do
        for _, item in ipairs(listOf(instance.items)) do
            if item.pending then out[#out + 1] = item end
        end
    end
    return out
end

local function retryPending()
    retry.scheduled = false
    retry.left = retry.left - 1
    local waiting = 0
    for _, item in ipairs(pendingItems()) do
        describe(item, item.id and ("item:" .. item.id))
        if item.pending then waiting = waiting + 1 end
    end
    if waiting == 0 then
        retry.left = 0
        chat("Dungeon loot: all items loaded. Type /mcp reload to save them.")
        Snapshot.windowCaptured(SECTION)
    elseif retry.left <= 0 then
        chat(("Dungeon loot: %d items are still loading; type /mcp loot again in a moment."):format(waiting))
    end
end

-- Item data arrives one item at a time, so one delayed read covers many items.
local function onItemData()
    if retry.left <= 0 or retry.scheduled then return end
    retry.scheduled = true
    local after = ns.Compat.resolve("C_Timer.After")
    if after then
        after(RETRY_SECONDS, function() pcall(retryPending) end)
    else
        retryPending()
    end
end

-- What this /mcp loot found, saved with the section: outcome is read, noMatch, readFailed or unavailable.
local function recordAttempt(outcome, query, journal, extra)
    local attempt = { at = time(), query = query, outcome = outcome, instancesInJournal = #journal.instances,
                      tiers = journal.tiers, inInstance = EJ.isInInstance(), instanceNames = {} }
    for i = 1, math.min(#journal.instances, MAX_NAMES) do attempt.instanceNames[i] = journal.instances[i].name end
    for key, value in pairs(extra or {}) do attempt[key] = value end
    section().lastAttempt = attempt
    Snapshot.windowCaptured(SECTION)
end

local EMPTY = { instances = {} }

local function reportUnavailable(missing, query)
    local saved = section()
    if #saved.instances == 0 then
        saved.unavailable, saved.checkedAt = "no " .. missing .. " in this client", time()
    end
    recordAttempt("unavailable", query, EMPTY, { error = "no " .. missing .. " in this client" })
    chat("This client does not give addons the Encounter Journal (no " .. missing .. "), so dungeon loot cannot be read."
        .. " If opening the Encounter Journal window first changes that, type /mcp loot again; otherwise /mcp probe"
        .. " lists what the client has. Type /mcp reload to save what was found.")
end

-- The journal lists no dungeon at all. The addon never loads the journal's own addon; the player opens the window.
local function reportEmptyJournal(query, journal)
    local extra = {}
    local text = "The Encounter Journal lists no dungeons."
    if EJ.notLoadedYet() then
        extra.hint = "The journal has no data yet: its addon (Blizzard_EncounterJournal) is not loaded. Open the Encounter"
            .. " Journal window once with the game's own UI, then type /mcp loot again."
        text = "The Encounter Journal has no data yet. Open the Encounter Journal window once (the game's own UI), then type"
            .. " /mcp loot again."
    end
    extra.journalLoaded = not EJ.notLoadedYet()
    recordAttempt("noMatch", query, journal, extra)
    chat(text .. " Type /mcp reload to save this for the assistant.")
end

local function readTargets(targets, ranges)
    local items, waiting, restored, failure, reads = 0, 0, true, nil, 0
    for i = 1, math.min(#targets, MAX_PER_COMMAND) do
        local instance = targets[i]
        local read, err = EJ.read(instance, MAX_ITEMS)
        if read then
            local entry = buildEntry(instance, read, ranges)
            remember(entry)
            reads = reads + 1
            restored = restored and read.restored
            items = items + #entry.items
            for _, item in ipairs(entry.items) do
                if item.pending then waiting = waiting + 1 end
            end
            local cut = ns.Compat.pick(read.truncated, " (only the first " .. MAX_ITEMS .. " are kept)", "")
            chat(("Encounter Journal read: %s - %d bosses, %d items%s."):format(instance.name, #entry.encounters,
                #entry.items, cut))
        else
            failure = failure or (instance.name .. ": " .. err)
            chat("Encounter Journal: " .. instance.name .. " could not be read (" .. err .. ").")
        end
    end
    return items, waiting, restored, failure, reads
end

local function namesOf(instances, from)
    local names = {}
    for i = from, #instances do names[#names + 1] = instances[i].name end
    return table.concat(names, ", ")
end

local function reportRead(targets, items, waiting, restored, failure)
    if #targets > MAX_PER_COMMAND then
        chat("Not read this time (at most " .. MAX_PER_COMMAND .. " per command): " .. namesOf(targets, MAX_PER_COMMAND + 1)
            .. ". Type /mcp loot <name> for them.")
    end
    if not restored then
        chat("The Encounter Journal's selection or loot filter could not be put back; reopen the journal to reset it.")
    end
    if waiting > 0 then
        retry.left = MAX_RETRIES
        chat(("%d items are still loading from the game; the addon reads them again as their data arrives."):format(waiting))
    end
    chat("Type /mcp reload to save the loot for the assistant.")
end

local function usage(query, journal)
    if query ~= "" then
        chat("No dungeon in the Encounter Journal matches \"" .. query .. "\".")
    else
        chat("You are not in a dungeon and no dungeon in the journal matches your level (the level ranges come from the"
            .. " last /mcp reload).")
    end
    chat("Usage: /mcp loot <dungeon name>, e.g. /mcp loot deadmines. Type /mcp reload to save what the journal offers.")
    recordAttempt("noMatch", query, journal)
end

-- /mcp loot [dungeon]: reads the journal once, on the player's request.
function DungeonLoot.run(rest)
    if Snapshot.isDisabled(SECTION) then
        chat("The dungeonLoot section is disabled. Enable it: /mcp on dungeonLoot")
        return
    end
    local query = tostring(rest or ""):match("^%s*(.-)%s*$")
    local missing = EJ.missingApi()
    if missing then
        reportUnavailable(missing, query)
        return
    end
    local instances, tiers = EJ.instances()
    local journal = { instances = instances, tiers = tiers }
    if #instances == 0 then
        reportEmptyJournal(query, journal)
        return
    end
    local ranges = savedLevelRanges()
    local targets = chooseTargets(query, instances, ranges)
    if #targets == 0 then
        usage(query, journal)
        return
    end
    local items, waiting, restored, failure, reads = readTargets(targets, ranges)
    local outcome = ns.Compat.pick(reads == 0, "readFailed", "read")
    recordAttempt(outcome, query, journal, { itemsRead = items, error = failure })
    reportRead(targets, items, waiting, restored, failure)
end

ns.Snapshot.addWindowSection(SECTION)
for _, event in ipairs({ "GET_ITEM_INFO_RECEIVED", "ITEM_DATA_LOAD_RESULT", "EJ_LOOT_DATA_RECIEVED" }) do
    ns.Events.on(event, onItemData)
end
