-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section probe (diagnostics, only via /mcp probe): list of client API functions
-- in selected namespaces and the registration result of the addon's events (events).
-- Used to find out what a function is called in this client and whether it knows a given event. Since 0.22.0 also
-- the raw answers of C_PetInfo.CanPetEatItem for a few bag items (petFood) and the Beast Lore tooltip of the target
-- (beastLore); the taming and Beast Lore events are in events, since the addon registers them.

local _, ns = ...

local NAMESPACES = {
    "C_PetInfo", "C_StableInfo", "C_Trainer", "C_SkillInfo", "C_Item", "C_TooltipInfo",
    "C_Container", "C_MerchantFrame", "C_Reputation", "C_ActionBar", "C_Bank", "C_TaxiMap",
    "C_UnitAuras", "C_PaperDollInfo", "C_Macro", "C_SpellBook", "C_Spell", "C_QuestLog",
    "C_Traits", "C_ClassTalents", "C_TradeSkillUI", "C_GameRules", "C_CurrencyInfo", "C_Mail", "C_Timer",
    "C_AuctionHouse", "C_LFGList", "C_PartyInfo", "C_EncounterJournal", "C_QuestInfoSystem", "TooltipDataProcessor",
    "C_EquipmentSet", "C_FriendList", "C_GuildInfo", "C_PvP", "C_Map", "C_BattleNet",
    "C_AchievementInfo", "C_MountJournal", "C_PetJournal", "C_TitleManager", "C_AddOns",
}

local GLOBAL_PREFIXES = {
    "^GetPet", "^HasPet", "^PetCan", "^PetHas", "^GetStable", "^GetTrainer", "^GetNumTrainer",
    "^GetMerchant", "^GetInventory", "^UnitStat", "^UnitRanged", "^GetNumStable", "^Taxi", "^NumTaxi",
    "^GetTradeSkill", "^GetCraft", "^GetNumCrafts", "^GetInbox", "^GetCurrency", "^GetQuestGreen", "^RequestTimePlayed",
    "^GetAuction", "^GetNumAuction", "^GetLFG", "^GetLFD", "^GetSavedInstance", "^GetNumSavedInstances", "^GetRaidRoster",
    "^GetTalent", "^GetNumTalent", "^UnitCharacterPoints", "^GetInspect", "^GetLoot",
    "^GetQuestLog", "^GetNumQuestLog", "^SelectQuestLog", "^GetQuestItem", "^GetNumQuest", "^GetReward",
    "^GetBuyback", "^GetNumBuyback", "^GetEquipmentSet", "^GetNumEquipmentSet", "^GetFriendInfo", "^GetNumFriends",
    "^GetGuild", "^GetNumGuild", "^IsInGuild", "^GetPVP", "^UnitHonor", "^UnitPVP", "^GetHonor", "^GetCursorPosition",
    "^GetXPExhaustion", "^UnitXP",
    "^GetTitleText", "^GetQuestID", "^GetObjectiveText", "^CanMerchant", "^GetRepair", "^GetTaxi",
    -- Weapon format strings are variables, not functions.
    "^GetAchievement", "^GetCategory", "^GetTotalAchievement", "^GetNumCompletedAchievements",
    "^GetLatestCompletedAchievements", "^GetPreviousAchievement", "^GetNextAchievement", "^GetStatistic",
    "^GetComparisonStatistic", "^GetNumTitles", "^IsTitleKnown", "^GetTitleName", "^GetCurrentTitle",
    "^GetNumCompanions", "^GetCompanionInfo",
    "^EJ_", "^BNGet", "^IsInInstance", "^GetInstanceInfo", "^IsAddOnLoaded", "^GetAddOnInfo",
}

local GLOBAL_FRAGMENTS = { "Loyal", "Happiness", "TrainingPoint", "PetFood", "PetExperience", "CritChance" }

local function functionNames(object)
    local names = {}
    for key, fn in pairs(object) do
        if type(key) == "string" and type(fn) == "function" then
            names[#names + 1] = key
        end
    end
    table.sort(names)
    return names
end

local function isProbedGlobal(name)
    for _, pattern in ipairs(GLOBAL_PREFIXES) do
        if name:find(pattern) then return true end
    end
    for _, fragment in ipairs(GLOBAL_FRAGMENTS) do
        if name:find(fragment, 1, true) then return true end
    end
    return false
end

-- Events the addon does not use yet: the probe only asks whether this client knows them.
local EVENT_CANDIDATES = { "PLAYER_CAMPING", "PLAYER_QUITING", "PLAYER_LEAVING_WORLD", "LOGOUT_CANCEL" }

local function candidateEvents()
    local frame, known = CreateFrame("Frame"), {}
    for _, event in ipairs(EVENT_CANDIDATES) do
        known[event] = pcall(frame.RegisterEvent, frame, event) and true or false
    end
    pcall(frame.UnregisterAllEvents, frame)
    return known
end

-- Blizzard UI addons whose state matters to a section: loaded now, loadable on demand, and whether their main
-- frame exists. The probe only asks; it loads nothing.
local UI_ADDONS = { Blizzard_EncounterJournal = "EncounterJournal" }

local function addonStates()
    local states = {}
    for name, frameName in pairs(UI_ADDONS) do
        local state = { frame = _G[frameName] ~= nil }
        state.loaded = ns.Compat.firstValue({ "C_AddOns.IsAddOnLoaded", "IsAddOnLoaded" }, name)
        state.loadable = ns.Compat.firstValue({ "C_AddOns.IsAddOnLoadable" }, name)
        if state.loadable == nil then
            local info = ns.Compat.callFirst({ "C_AddOns.GetAddOnInfo", "GetAddOnInfo" }, name)
            state.loadable = info and ns.Compat.plain(info[4])
        end
        states[name] = state
    end
    return states
end

local function collectProbe()
    local probe = { capturedAt = time(), namespaces = {}, globals = {}, events = ns.Events.registrationResults() }
    probe.eventCandidates = candidateEvents()
    probe.addons = addonStates()
    local okFood, petFood = pcall(ns.PetFood.probe)
    probe.petFood = okFood and petFood or nil
    local okLore, beastLore = pcall(ns.BeastLore.probeTarget)
    probe.beastLore = okLore and beastLore or nil
    for _, namespace in ipairs(NAMESPACES) do
        local object = _G[namespace]
        if type(object) == "table" then
            probe.namespaces[namespace] = functionNames(object)
        end
    end
    for key, fn in pairs(_G) do
        if type(key) == "string" and type(fn) == "function" and isProbedGlobal(key) then
            probe.globals[#probe.globals + 1] = key
        end
    end
    table.sort(probe.globals)
    return probe
end

ns.Snapshot.addWindowSection("probe", collectProbe, nil, { quiet = true })
