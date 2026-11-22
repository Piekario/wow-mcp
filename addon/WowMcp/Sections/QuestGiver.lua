-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Sections questOffer and questTurnIn (from the window): the quest an NPC is just offering (QUEST_DETAIL),
-- and the reward choices when turning in a quest (QUEST_COMPLETE). We keep only the latest of each.
-- The addon accepts, turns in and chooses nothing; it only reads what the window already shows.

local _, ns = ...
local plain, firstValue = ns.Compat.plain, ns.Compat.firstValue

local function questGiverName()
    return plain(UnitName("questnpc")) or plain(UnitName("npc"))
end

local function nonEmptyText(text)
    if type(text) == "string" and text:find("%S") then return text end
    return nil
end

-- Shared part of both windows: title, ID, NPC, rewards and time.
local function questWindow(textPaths)
    local title = firstValue({ "GetTitleText" })
    if not title then return nil end
    return {
        title = title,
        id = firstValue({ "GetQuestID" }),
        text = nonEmptyText(firstValue(textPaths)),
        npc = questGiverName(),
        rewards = ns.QuestRewards.fromGiver(),
        capturedAt = time(),
    }
end

-- Quest offer: text is the quest objectives (GetObjectiveText).
local function collectQuestOffer()
    return questWindow({ "GetObjectiveText" })
end

-- Quest turn-in: text is the NPC's text at the reward (GetRewardText).
local function collectQuestTurnIn()
    return questWindow({ "GetRewardText" })
end

ns.Snapshot.addWindowSection("questOffer", collectQuestOffer, { "QUEST_DETAIL" })
ns.Snapshot.addWindowSection("questTurnIn", collectQuestTurnIn, { "QUEST_COMPLETE" })
