-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Guide: the leveling guide the player follows in the Guidelime addon (for example a TUGs guide) and how far
-- they are in it, so the assistant can answer "what is my next step" from the guide instead of from memory.
--
-- Guidelime keeps its parsed guides private. What another addon can see is:
--   * the guide texts, at the moment a guide addon registers them (Guidelime.registerGuide; we listen with
--     hooksecurefunc, which runs after the original and cannot change it), and
--   * GuidelimeDataChar: the name of the current guide and the steps the player ticked or skipped by hand.
-- Quest steps Guidelime completes by itself are not stored there, so their state is read from the quest log.
-- WowMcp ships no guide text; the dump holds only the next few steps of the player's own installed guide.
--
-- Only reads. Nothing here calls into Guidelime or changes its data.

local _, ns = ...
local plain = ns.Compat.plain

local Guide = {}
ns.Guide = Guide

Guide.MAX_NEXT_STEPS = 15
Guide.MAX_PASSED_STEPS = 5  -- open steps before the furthest done step that may take places in the list
Guide.MAX_STEP_TEXT = 400

local registered = {}   -- { text, group } in registration order
local lastQuestIDs = {} -- quest IDs of the steps listed by the last collect()

function Guide.remember(text, group)
    if type(text) ~= "string" then return end
    registered[#registered + 1] = { text = text, group = type(group) == "string" and group or nil }
end

function Guide.registeredCount()
    return #registered
end

-- Listens for guide registrations. Guidelime must already be loaded (## OptionalDeps in the .toc), and the
-- guide addons, which depend on Guidelime, load after it.
function Guide.listen()
    local guidelime = _G.Guidelime
    if type(guidelime) ~= "table" or type(guidelime.registerGuide) ~= "function" then return false end
    if type(hooksecurefunc) ~= "function" then return false end
    return (pcall(hooksecurefunc, guidelime, "registerGuide", Guide.remember))
end

local function squeeze(text)
    return (tostring(text or ""):gsub("%s+", ""))
end

-- Content of the first [<code>...] tag in the text, e.g. tagContent(text, "NX").
local function tagContent(text, code)
    return text:match("%[" .. code .. "%s*([^%]]-)%s*%]")
end

-- Guidelime names a guide "<group> <min>-<max> <title>" from the group and the [N...] tag. Whitespace is
-- ignored in the comparison, so the exact spacing rules do not matter.
local function guideName(entry)
    local group = entry.group or ""
    if group:sub(1, 10) == "Guidelime_" then group = group:sub(11) end
    local title = entry.text:match("%[N([^X%]][^%]]*)%]") or ""
    return squeeze(group .. title)
end

function Guide.findText(name)
    local wanted = squeeze(name)
    if wanted == "" then return nil end
    for i = #registered, 1, -1 do -- a guide registered twice: the last one wins, as in Guidelime
        if guideName(registered[i]) == wanted then return registered[i].text end
    end
    return nil
end

local HEADER_CODES = { GA = true, N = true, NX = true, D = true, DL = true }
local QUEST_ACTIONS = { A = "accept", P = "accept", C = "complete", W = "complete", T = "turnin", S = "skip" }

-- Non-empty lines of a guide with their line numbers. Guidelime counts every line break (also the two of a
-- CRLF pair) and keys the player's progress by that number, so the counting here must match it.
function Guide.lines(text)
    local out, number = {}, 1
    local rest = text:gsub("([^\n\r]-)[\n\r]", function(line)
        if line ~= "" then out[#out + 1] = { line = number, text = line } end
        number = number + 1
        return ""
    end)
    if rest ~= "" then out[#out + 1] = { line = number, text = rest } end
    return out
end

local function questTags(text)
    local quests = {}
    for code, id, rest in text:gmatch("%[Q(%u)%s*(%d+)([^%]]*)%]") do
        local action = QUEST_ACTIONS[code]
        if action then
            quests[#quests + 1] = { id = tonumber(id), action = action, objective = tonumber(rest:match("^,%s*(%d+)")) }
        end
    end
    return quests
end

-- A step from one guide line; nil for a header line (guide name, next guide, description).
function Guide.parseStep(entry)
    local code = entry.text:match("^%s*%[(%u+)")
    if code and HEADER_CODES[code] then return nil end
    local step = { line = entry.line, text = entry.text, quests = questTags(entry.text) }
    step.applies = tagContent(entry.text, "A%s")
    if entry.text:find("%[OC?%]") then step.optional = true end
    return step
end

local RACES = {
    human = true, dwarf = true, nightelf = true, gnome = true, orc = true, undead = true, tauren = true, troll = true,
    draenei = true, bloodelf = true, worgen = true, goblin = true, pandaren = true,
}
local CLASSES = {
    warrior = true, paladin = true, hunter = true, rogue = true, priest = true, shaman = true, mage = true,
    warlock = true, druid = true, deathknight = true, monk = true,
}
local FACTIONS = { alliance = true, horde = true }
local RACE_TOKEN_NAMES = { scourge = "undead" } -- the client's token for Undead

local function token(value)
    return squeeze(plain(value) or ""):lower()
end

-- The player's race, class and faction as the lower-case English words guides use.
function Guide.player()
    local _, raceToken = UnitRace("player")
    local _, classToken = UnitClass("player")
    local race = token(raceToken)
    return { race = RACE_TOKEN_NAMES[race] or race, class = token(classToken), faction = token(UnitFactionGroup("player")) }
end

-- Whether a step restricted by [A Orc, Troll] (races, classes, a faction) is for this player. Races and
-- classes restrict separately; words we do not know restrict nothing.
function Guide.appliesTo(applies, player)
    if type(applies) ~= "string" then return true end
    local races, classes, factions = {}, {}, {}
    for word in applies:gmatch("[^,]+") do
        word = squeeze(word):lower()
        if RACES[word] then races[#races + 1] = word
        elseif CLASSES[word] then classes[#classes + 1] = word
        elseif FACTIONS[word] then factions[#factions + 1] = word end
    end
    local function allows(list, value)
        if #list == 0 then return true end
        for _, word in ipairs(list) do
            if word == value then return true end
        end
        return false
    end
    return allows(races, player.race) and allows(classes, player.class) and allows(factions, player.faction)
end

-- Quests in the log: { [questID] = "complete" | "active" }.
function Guide.logState()
    local state = {}
    if C_QuestLog and C_QuestLog.GetNumQuestLogEntries and C_QuestLog.GetInfo then
        for i = 1, C_QuestLog.GetNumQuestLogEntries() or 0 do
            local info = C_QuestLog.GetInfo(i)
            local id = info and not info.isHeader and plain(info.questID)
            if id then
                local complete = C_QuestLog.IsComplete and C_QuestLog.IsComplete(id)
                state[id] = complete and "complete" or "active"
            end
        end
    elseif GetNumQuestLogEntries and GetQuestLogTitle then
        for i = 1, GetNumQuestLogEntries() or 0 do
            local _, _, _, isHeader, _, isComplete, _, questID = GetQuestLogTitle(i)
            questID = plain(questID)
            if not isHeader and questID then
                state[questID] = (isComplete == 1 or isComplete == true) and "complete" or "active"
            end
        end
    end
    return state
end

local function turnedIn(questID)
    local results = ns.Compat.callFirst({ "C_QuestLog.IsQuestFlaggedCompleted", "IsQuestFlaggedCompleted" }, questID)
    return results ~= nil and results[1] == true
end

local function objectiveFinished(questID, index)
    if not (index and C_QuestLog and C_QuestLog.GetQuestObjectives) then return false end
    local ok, objectives = pcall(C_QuestLog.GetQuestObjectives, questID)
    local objective = ok and type(objectives) == "table" and objectives[index]
    return type(objective) == "table" and objective.finished == true
end

-- Whether the quest part of a step is already behind the player.
local function questDone(quest, log)
    if quest.action == "skip" then return true end
    if turnedIn(quest.id) then return true end
    if quest.action == "accept" then return log[quest.id] ~= nil end
    if quest.action == "complete" then
        return log[quest.id] == "complete" or (log[quest.id] ~= nil and objectiveFinished(quest.id, quest.objective))
    end
    return false -- turnin: only a quest flagged completed counts
end

local function allQuestsDone(step, log)
    if #step.quests == 0 then return false end
    for _, quest in ipairs(step.quests) do
        if not questDone(quest, log) then return false end
    end
    return true
end

local function tickedByPlayer(progress, line)
    local entry = type(progress) == "table" and progress[line]
    return type(entry) == "table" and (entry.completed == true or entry.skip == true)
end

local function clip(text)
    if #text <= Guide.MAX_STEP_TEXT then return text end
    return ns.Note.clip(text, Guide.MAX_STEP_TEXT)
end

-- The player's steps of a guide text with their state, and the counts for the summary.
function Guide.progress(text, progress, player, log)
    local steps, notForPlayer = {}, 0
    local lastDone = 0
    for _, entry in ipairs(Guide.lines(text)) do
        local step = Guide.parseStep(entry)
        if step then
            if Guide.appliesTo(step.applies, player) then
                step.done = tickedByPlayer(progress, step.line) or allQuestsDone(step, log)
                steps[#steps + 1] = step
                if step.done then lastDone = #steps end
            else
                notForPlayer = notForPlayer + 1
            end
        end
    end
    -- A step without a quest (travel, a hint) leaves no trace when it is done: once a later step is done,
    -- the player has passed it.
    local done = 0
    for index, step in ipairs(steps) do
        if not step.done and #step.quests == 0 and index < lastDone then step.done = true end
        if step.done then done = done + 1 end
        step.passed = (not step.done and index < lastDone) or nil
    end
    return steps, done, notForPlayer
end

local function collectGuide()
    lastQuestIDs = {}
    local data = _G.GuidelimeDataChar
    if type(_G.Guidelime) ~= "table" or type(data) ~= "table" then return nil end
    local name = plain(data.currentGuide)
    if type(name) ~= "string" or name == "" then return { addon = "Guidelime" } end
    local guide = { addon = "Guidelime", name = name }
    local text = Guide.findText(name)
    if not text then
        guide.textUnavailable = true
        guide.registeredGuides = #registered
        return guide
    end
    guide.nextGuide = tagContent(text, "NX")
    local progress = type(data.progress) == "table" and data.progress[name] or nil
    local steps, done, notForPlayer = Guide.progress(text, progress, Guide.player(), Guide.logState())
    guide.totalSteps = #steps
    guide.doneSteps = done
    guide.notForPlayer = notForPlayer > 0 and notForPlayer or nil
    -- Open steps before the furthest done one (skipped or left for later) must not push the current step
    -- off the list: only the first few are listed, the rest are counted.
    guide.next = {}
    local passed = 0
    for _, step in ipairs(steps) do
        if not step.done then
            if step.passed then passed = passed + 1 end
            local listed = #guide.next < Guide.MAX_NEXT_STEPS and (not step.passed or passed <= Guide.MAX_PASSED_STEPS)
            if listed then
                guide.next[#guide.next + 1] = {
                    line = step.line,
                    text = clip(step.text),
                    quests = #step.quests > 0 and step.quests or nil,
                    optional = step.optional,
                    passed = step.passed,
                }
                for _, quest in ipairs(step.quests) do lastQuestIDs[#lastQuestIDs + 1] = quest.id end
            end
        end
    end
    guide.passedSteps = passed > 0 and passed or nil
    return guide
end

-- Quest IDs named by the steps of the last collected guide section (for the quest database section).
function Guide.pendingQuestIDs()
    return lastQuestIDs
end

Guide.listen()

ns.Snapshot.addSection("guide", collectGuide)
