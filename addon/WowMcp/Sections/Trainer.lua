-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section trainer (from the window): the trainer's offer (also pet and profession trainers).

local _, ns = ...
local plain = ns.Compat.plain

local KNOWN_CATEGORIES = { available = true, unavailable = true, used = true, header = true }

-- The order of values differs between client versions, so we look for the category and rank by content.
-- Strings with a path (slash) are icons, not ranks.
local function serviceCategoryAndRank(returns)
    local category, rank
    for j = 2, 6 do
        local v = plain(returns[j])
        if type(v) == "string" then
            if KNOWN_CATEGORIES[v] then
                category = category or v
            elseif v ~= "" and not rank and not v:find("[\\/]") then
                rank = v
            end
        end
    end
    return category, rank
end

local function trainerService(i, name, rank, category)
    local service = { name = name, rank = rank, category = category }
    if GetTrainerServiceCost then
        service.costCopper = plain((GetTrainerServiceCost(i)))
    end
    if GetTrainerServiceLevelReq then
        service.levelRequired = plain((GetTrainerServiceLevelReq(i)))
    end
    return service
end

local function collectTrainer()
    if not GetNumTrainerServices or not GetTrainerServiceInfo then return nil end
    local count = plain(GetNumTrainerServices())
    if type(count) ~= "number" or count <= 0 then return nil end

    local trainer = { npc = plain(UnitName("npc")), capturedAt = time(), services = {} }
    for i = 1, count do
        local returns = { GetTrainerServiceInfo(i) }
        local name = plain(returns[1])
        local category, rank = serviceCategoryAndRank(returns)
        if name and category ~= "header" then
            trainer.services[#trainer.services + 1] = trainerService(i, name, rank, category)
        end
    end
    return trainer
end

-- trainer is the last trainer, trainers: several latest (NPC name -> the same as trainer).
ns.Snapshot.addWindowSection("trainer", collectTrainer, { "TRAINER_SHOW", "TRAINER_UPDATE" },
    { history = { name = "trainers", key = "npc", max = 10 } })
