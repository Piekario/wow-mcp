-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section actionBars: what sits under which button. Helps with building rotations.

local _, ns = ...
local plain, callFirst, firstValue = ns.Compat.plain, ns.Compat.callFirst, ns.Compat.firstValue

local ACTION_SLOTS = 180
local BUTTONS_PER_BAR = 12

-- Name of what is in the slot: a spell, item or macro; otherwise the button text.
local function actionName(slot, kind, id)
    local name
    if kind == "spell" and id then
        name = firstValue({ "C_Spell.GetSpellName" }, id)
    elseif kind == "item" and id then
        name = firstValue({ "C_Item.GetItemNameByID", "C_Item.GetItemInfo", "GetItemInfo" }, id)
    elseif kind == "macro" and id then
        name = firstValue({ "GetMacroInfo" }, id)
    end
    return name or firstValue({ "GetActionText" }, slot)
end

local function action(slot)
    local info = callFirst({ "GetActionInfo", "C_ActionBar.GetActionInfo" }, slot)
    local kind, id = info and plain(info[1]), info and plain(info[2])
    local entry = {
        slot = slot,
        bar = math.floor((slot - 1) / BUTTONS_PER_BAR) + 1,
        button = (slot - 1) % BUTTONS_PER_BAR + 1,
        kind = kind,
        id = id,
        name = actionName(slot, kind, id),
    }
    -- ACTIONBUTTON1-12 bindings apply only to the main bar.
    if slot <= BUTTONS_PER_BAR then
        entry.key = firstValue({ "GetBindingKey" }, "ACTIONBUTTON" .. slot)
    end
    return entry
end

local function collectActionBars()
    local actions = {}
    for slot = 1, ACTION_SLOTS do
        local has = callFirst({ "HasAction", "C_ActionBar.HasAction" }, slot)
        if has and has[1] then
            actions[#actions + 1] = action(slot)
        end
    end
    return actions
end

ns.Snapshot.addSection("actionBars", collectActionBars)
