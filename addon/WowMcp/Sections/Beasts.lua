-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section beasts (Hunters only): the beasts this account tamed or examined with Beast Lore (Beasts.lua), most recent
-- first. The observations are account-wide, so every Hunter's dump carries all of them.

local _, ns = ...

local function collectBeasts()
    if not ns.Beasts.isHunter() then return nil end
    return ns.Beasts.list()
end

ns.Snapshot.addSection("beasts", collectBeasts)
