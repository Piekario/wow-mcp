-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section recipes (from the window): recipes of the open profession, saved under its name.
-- Each profession overwrites only its own entry; others stay from earlier visits.

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue
local pick = ns.Compat.pick
local Items = ns.Items

-- Recipe color: whether it still gives skill points.
-- optimal = orange (always), medium = yellow, easy = green, trivial = grey (never).
local DIFFICULTY_BY_INDEX = { [0] = "optimal", [1] = "medium", [2] = "easy", [3] = "trivial" }

-- Reagent read shared by the profession window (TradeSkill) and the Craft window (Enchanting, Beast Training).
local function readReagents(countFn, infoFn, index)
    local count = firstValue({ countFn }, index)
    if type(count) ~= "number" or count <= 0 then return nil end
    local reagents = {}
    for j = 1, count do
        local r = callFirst({ infoFn }, index, j)
        if r and plain(r[1]) then
            reagents[#reagents + 1] = { name = r[1], need = plain(r[3]), have = plain(r[4]) }
        end
    end
    return reagents
end

local function positiveNumber(value)
    if type(value) == "number" and value > 0 then return value end
    return nil
end

-- Craft: name, subName, type, numAvailable, isExpanded, trainingPointCost, requiredLevel.
local function addCraftDetails(recipe, info, index)
    local rank = plain(info[2])
    if rank and rank ~= "" then recipe.rank = rank end
    recipe.trainingPoints = positiveNumber(plain(info[6]))
    recipe.levelRequired = positiveNumber(plain(info[7]))
    recipe.reagents = readReagents("GetCraftNumReagents", "GetCraftReagentInfo", index)
end

-- TradeSkill: name, type, numAvailable; item and cooldown from separate functions.
local function addTradeSkillDetails(recipe, index)
    recipe.reagents = readReagents("GetTradeSkillNumReagents", "GetTradeSkillReagentInfo", index)
    local link = firstValue({ "GetTradeSkillItemLink" }, index)
    recipe.itemId = Items.idFromLink(link)
    local cd = firstValue({ "GetTradeSkillCooldown" }, index)
    if type(cd) == "number" and cd > 0 then
        recipe.cooldownSeconds = math.floor(cd)
        recipe.readyAt = time() + math.floor(cd)
    end
end

-- Old-style profession window (GetTradeSkillInfo) or the Craft window (GetCraftInfo).
-- Collapsed headers hide recipes, so it is good to have them expanded on open.
local function collectClassicRecipes(kind)
    local isCraft = kind == "craft"
    local line = callFirst({ isCraft and "GetCraftDisplaySkillLine" or "GetTradeSkillLine" })
    local name = line and plain(line[1])
    if not name or name == "UNKNOWN" then return nil end
    local count = firstValue({ isCraft and "GetNumCrafts" or "GetNumTradeSkills" })
    if type(count) ~= "number" or count == 0 then return nil end

    local profession = {
        name = name, rank = plain(line[2]), maxRank = plain(line[3]),
        capturedAt = time(), recipes = {},
    }
    local header
    for index = 1, count do
        local info = callFirst({ isCraft and "GetCraftInfo" or "GetTradeSkillInfo" }, index)
        if info and plain(info[1]) then
            local recipeName = info[1]
            local difficulty = plain(pick(isCraft, info[3], info[2]))
            local available = plain(pick(isCraft, info[4], info[3]))
            if difficulty == "header" then
                header = recipeName
            else
                local recipe = {
                    name = recipeName, difficulty = difficulty, group = header,
                    canCraft = positiveNumber(available),
                }
                if isCraft then
                    addCraftDetails(recipe, info, index)
                else
                    addTradeSkillDetails(recipe, index)
                end
                profession.recipes[#profession.recipes + 1] = recipe
            end
        end
    end
    return profession
end

-- Reagents, item and cooldown of a recipe from the new API (C_TradeSkillUI.GetRecipeSchematic,
-- GetRecipeCooldown). Only reads data of the open window; crafts nothing.
local REAGENT_TYPE_BASIC = 1 -- Enum.CraftingReagentType.Basic; optional extras are skipped

local function modernReagents(schematic)
    local slots = type(schematic.reagentSlotSchematics) == "table" and schematic.reagentSlotSchematics or {}
    local reagents = {}
    for _, slot in ipairs(slots) do
        local kind = plain(slot.reagentType)
        local first = type(slot.reagents) == "table" and slot.reagents[1]
        local id = type(first) == "table" and plain(first.itemID)
        if id and (kind == nil or kind == REAGENT_TYPE_BASIC) then
            reagents[#reagents + 1] = {
                id = id,
                name = firstValue({ "C_Item.GetItemNameByID", "GetItemInfo" }, id),
                need = plain(slot.quantityRequired),
                have = firstValue({ "C_Item.GetItemCount", "GetItemCount" }, id),
            }
        end
    end
    if #reagents == 0 then return nil end
    return reagents
end

local function addModernDetails(recipe, id)
    local schematic = callFirst({ "C_TradeSkillUI.GetRecipeSchematic" }, id, false)
    if schematic and type(schematic[1]) == "table" then
        recipe.itemId = plain(schematic[1].outputItemID)
        recipe.reagents = modernReagents(schematic[1])
    end
    local cd = firstValue({ "C_TradeSkillUI.GetRecipeCooldown" }, id)
    if type(cd) == "number" and cd > 0 then
        recipe.cooldownSeconds = math.floor(cd)
        recipe.readyAt = time() + math.floor(cd)
    end
end

-- Profession window in the new API (C_TradeSkillUI).
local function collectModernRecipes()
    if not (C_TradeSkillUI and C_TradeSkillUI.GetAllRecipeIDs and C_TradeSkillUI.GetRecipeInfo) then return nil end
    local base = callFirst({ "C_TradeSkillUI.GetChildProfessionInfo", "C_TradeSkillUI.GetBaseProfessionInfo" })
    local info = base and type(base[1]) == "table" and base[1]
    local name = info and (plain(info.professionName) or plain(info.parentProfessionName))
    if not name then return nil end
    local profession = {
        name = name, rank = plain(info.skillLevel), maxRank = plain(info.maxSkillLevel),
        capturedAt = time(), recipes = {},
    }
    local okIDs, ids = pcall(C_TradeSkillUI.GetAllRecipeIDs)
    for _, id in ipairs(okIDs and ids or {}) do
        local ok, r = pcall(C_TradeSkillUI.GetRecipeInfo, id)
        if ok and type(r) == "table" and plain(r.name) and r.learned ~= false then
            local recipe = {
                id = id, name = r.name,
                difficulty = DIFFICULTY_BY_INDEX[r.relativeDifficulty] or plain(r.relativeDifficulty),
                canCraft = positiveNumber(plain(r.numAvailable)),
            }
            pcall(addModernDetails, recipe, id)
            profession.recipes[#profession.recipes + 1] = recipe
        end
    end
    if #profession.recipes == 0 then return nil end
    return profession
end

local function collectOpenProfession(kind)
    if kind == "craft" then return collectClassicRecipes("craft") end
    return collectClassicRecipes("trade") or collectModernRecipes()
end

local function captureRecipes(kind)
    local ok, profession = pcall(collectOpenProfession, kind)
    if not ok or not profession then return end
    local current = ns.Snapshot.current()
    current.recipes = (type(current.recipes) == "table" and current.recipes) or {}
    current.recipes[profession.name] = profession
    ns.Snapshot.windowCaptured("recipes")
end

-- TRADE_SKILL_UPDATE can arrive many times per second; we scan at most once a second.
local lastRecipeScan = {}
local function throttledRecipes(kind)
    local t = GetTime and GetTime() or time()
    if lastRecipeScan[kind] and t - lastRecipeScan[kind] < 1 then return end
    lastRecipeScan[kind] = t
    captureRecipes(kind)
end

local scanTradeSkill = ns.Snapshot.whenEnabled("recipes", function() throttledRecipes("trade") end)
local scanCraft = ns.Snapshot.whenEnabled("recipes", function() throttledRecipes("craft") end)

-- Forever 1.60.1 (per /mcp probe) accepts TRADE_SKILL_SHOW and TRADE_SKILL_LIST_UPDATE; TRADE_SKILL_UPDATE,
-- CRAFT_SHOW and CRAFT_UPDATE (and the classic GetTradeSkill*, GetCraft*) remain for older clients.
ns.Snapshot.addWindowSection("recipes")
for _, event in ipairs({ "TRADE_SKILL_SHOW", "TRADE_SKILL_UPDATE", "TRADE_SKILL_LIST_UPDATE" }) do
    ns.Events.on(event, scanTradeSkill)
end
for _, event in ipairs({ "CRAFT_SHOW", "CRAFT_UPDATE" }) do
    ns.Events.on(event, scanCraft)
end
