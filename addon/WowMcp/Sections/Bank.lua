-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section bank (from the window): bank contents. Bank container numbers differ between
-- client versions, so we check all known variants and take the ones that exist.

local _, ns = ...
local Items = ns.Items

local LEGACY_BANK_CONTAINER = -1 -- main part of the bank in older clients

local function bankContainerIndexes()
    local candidates, seen = {}, {}
    local function add(index)
        if type(index) == "number" and not seen[index] then
            seen[index] = true
            candidates[#candidates + 1] = index
        end
    end
    add(LEGACY_BANK_CONTAINER)
    local firstBankBag = Items.lastEquippedBag() + 1
    for index = firstBankBag, firstBankBag + (NUM_BANKBAGSLOTS or 7) - 1 do add(index) end
    if Enum and Enum.BagIndex then
        -- pairs() gives a random order, and the dump must be reproducible.
        local indexes = {}
        for name, index in pairs(Enum.BagIndex) do
            if type(name) == "string" and name:find("Bank") and type(index) == "number" then
                indexes[#indexes + 1] = index
            end
        end
        table.sort(indexes)
        for _, index in ipairs(indexes) do add(index) end
    end
    return candidates
end

local function collectBank()
    local candidates = bankContainerIndexes()
    local bank = { capturedAt = time(), containers = {} }
    for _, index in ipairs(candidates) do
        local ok, container = pcall(Items.scanContainer, index)
        if ok and container then bank.containers[#bank.containers + 1] = container end
    end
    if #bank.containers == 0 then return nil end
    return bank
end

ns.Snapshot.addWindowSection("bank", collectBank, { "BANKFRAME_OPENED" })
