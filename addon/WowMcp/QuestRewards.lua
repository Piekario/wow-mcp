-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Quest rewards: choosable items (choices), fixed items (items), money and experience.
-- Shared by the quest log (Sections/Quests) and the quest giver window (Sections/QuestGiver).
-- Read-only: the addon never chooses or turns in anything.

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue
local Items = ns.Items

local QuestRewards = {}
ns.QuestRewards = QuestRewards

local function positive(value)
    value = plain(value)
    if type(value) == "number" and value > 0 then return value end
    return nil
end

-- Reward item from results of the form name, texture, count, quality, isUsable, itemID.
local function rewardItem(info, link, readTooltip)
    info = info or {}
    if not plain(info[1]) and not plain(link) then return nil end
    local item = {
        name = plain(info[1]),
        count = positive(info[3]) or 1,
        quality = plain(info[4]),
        id = plain(info[6]),
    }
    if info[5] == false then item.usable = false end
    return Items.describe(item, plain(link), readTooltip)
end

-- Item list: count is the number of entries, read(i) returns { info, link, readTooltip }.
local function rewardList(count, read)
    local list = {}
    for i = 1, (type(count) == "number" and count or 0) do
        local info, link, readTooltip = read(i)
        local item = rewardItem(info, link, readTooltip)
        if item then list[#list + 1] = item end
    end
    if #list == 0 then return nil end
    return list
end

-- Currencies in the reward (new API): { name, id, count }.
local function rewardCurrencies(questID)
    local r = callFirst({ "C_QuestLog.GetQuestRewardCurrencies", "C_QuestInfoSystem.GetQuestRewardCurrencies" }, questID)
    if not r or type(r[1]) ~= "table" then return nil end
    local list = {}
    for _, c in ipairs(r[1]) do
        if type(c) == "table" and plain(c.name) then
            list[#list + 1] = { name = plain(c.name), id = plain(c.currencyID), count = plain(c.totalRewardAmount) }
        end
    end
    if #list == 0 then return nil end
    return list
end

-- Returns the rewards, or nil when the quest gives nothing (or the client supplied nothing).
local function nonEmpty(rewards)
    if next(rewards) == nil then return nil end
    return rewards
end

-- Reads the i-th reward entry of the quest with the given ID (new API).
local function byQuestID(paths, questID)
    return function(i)
        local info = callFirst(paths, i, questID) -- without the second result (function paths)
        return info
    end
end

-- The new API takes the quest ID, so there is no need to select the quest in the log.
function QuestRewards.fromLog(questID)
    if not questID then return nil end
    return nonEmpty({
        choices = rewardList(firstValue({ "GetNumQuestLogChoices" }, questID), byQuestID({ "GetQuestLogChoiceInfo" }, questID)),
        items = rewardList(firstValue({ "GetNumQuestLogRewards" }, questID), byQuestID({ "GetQuestLogRewardInfo" }, questID)),
        money = positive(firstValue({ "GetQuestLogRewardMoney" }, questID)),
        xp = positive(firstValue({ "GetQuestLogRewardXP" }, questID)),
        currencies = rewardCurrencies(questID),
    })
end

-- The old API reads rewards of the selected quest. We select an entry only for the duration of the read
-- and restore the previous selection right away (the player sees no change).
-- Without GetQuestLogSelection we could not restore the selection, so then we skip the rewards.
-- Reads the i-th reward entry of the selected quest (old API) with the item link.
local function selectedQuestItem(kind, paths)
    return function(i) return callFirst(paths, i), firstValue({ "GetQuestLogItemLink" }, kind, i) end
end

local function selectedQuestRewards()
    return nonEmpty({
        choices = rewardList(firstValue({ "GetNumQuestLogChoices" }), selectedQuestItem("choice", { "GetQuestLogChoiceInfo" })),
        items = rewardList(firstValue({ "GetNumQuestLogRewards" }), selectedQuestItem("reward", { "GetQuestLogRewardInfo" })),
        money = positive(firstValue({ "GetQuestLogRewardMoney" })),
        xp = positive(firstValue({ "GetQuestLogRewardXP" })),
    })
end

function QuestRewards.fromLogEntry(questIndex)
    if not (GetQuestLogSelection and SelectQuestLogEntry) then return nil end
    local previous = GetQuestLogSelection()
    SelectQuestLogEntry(questIndex)
    local ok, rewards = pcall(selectedQuestRewards)
    SelectQuestLogEntry(previous or 0)
    if ok then return rewards end
    return nil
end

local function giverItem(kind)
    return function(i)
        local readTooltip = function() return ns.Compat.tooltipFor("GetQuestItem", kind, i) end
        return callFirst({ "GetQuestItemInfo" }, kind, i), firstValue({ "GetQuestItemLink" }, kind, i), readTooltip
    end
end

-- Rewards shown in the open quest window at an NPC.
function QuestRewards.fromGiver()
    return nonEmpty({
        choices = rewardList(firstValue({ "GetNumQuestChoices" }), giverItem("choice")),
        items = rewardList(firstValue({ "GetNumQuestRewards" }), giverItem("reward")),
        money = positive(firstValue({ "GetRewardMoney" })),
        xp = positive(firstValue({ "GetRewardXP" })),
    })
end
