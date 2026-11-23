-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- WowMcp: dumps character state to SavedVariables.
--
-- The addon ONLY reads data through the official API and writes it to the file
-- WTF/Account/<account>/SavedVariables/WowMcp.lua. It clicks nothing,
-- sends nothing and automates nothing. The game writes SavedVariables to disk on
-- /reload and on logout, so the data is as fresh as the last /reload.
--
-- Commands (Commands.lua):
--   /mcp          dump state (saved on the next /reload or logout)
--   /mcp reload   dump state and /reload immediately
--   /mcp status   when the last dump was taken and whether there were errors
--   /mcp probe    like /mcp reload, but adds a list of client API functions (diagnostics)
--   /mcp panel    shows or hides the panel with the dump age and a "Save" button
--   /mcp note     note for the assistant in the dump (without text: removes the note)
--   /mcp quest    <id or name>: where a quest starts and ends (from the installed Questie database)
--   /mcp ahtip    on|off: auction house price in the item tooltip
--   /mcp minimap on|off: minimap button (left click shows the panel)
--   /mcp compact on|off: one-line panel
--   /mcp off|on   disables or enables a dump section
--
-- The panel and the "Save" button do exactly the same as /mcp reload and work
-- only when the player clicks. The addon never saves anything on its own.
--
-- File layout (load order in WowMcp.toc, dependencies only point down the list):
--   Core, Compat, Maps, QuestDb, Items, QuestRewards, TalentGrid, TraitTree, Store, Events, Timing,
--   LogoutGuard, Snapshot, Periods, LegacyDiscovery - shared foundations
--   Sections/*                                    - one file per dump section;
--                                                   each registers itself in Snapshot
--   Panel, AuctionTooltip, WowMcp (lifecycle), Commands - player UI
--
-- The Forever client (Interface 16001) uses the new API (C_QuestLog, C_Container,
-- C_SpellBook, C_Traits). Each section also tries the older function names and
-- is wrapped in pcall: an error in one section does not break the rest, and its
-- text goes to the "errors" field of the dump.

local _, ns = ...

ns.ADDON_VERSION = "0.19.1"
ns.SCHEMA_VERSION = 2

function ns.chat(text)
    DEFAULT_CHAT_FRAME:AddMessage("|cff66ccff[WowMcp]|r " .. tostring(text))
end

function ns.round1(number)
    if type(number) ~= "number" then return nil end
    return math.floor(number * 10 + 0.5) / 10
end
