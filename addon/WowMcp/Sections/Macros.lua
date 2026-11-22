-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section macros: account and character macros with their bodies.

local _, ns = ...
local plain = ns.Compat.plain

local function collectMacros()
    if not (GetNumMacros and GetMacroInfo) then return nil end
    local macros = {}
    local accountCount, characterCount = GetNumMacros()
    -- Character macros start right after all the account macro slots.
    local accountSlots = MAX_ACCOUNT_MACROS or 120
    local function add(index, scope)
        local ok, name, _, body = pcall(GetMacroInfo, index)
        if ok and plain(name) then
            macros[#macros + 1] = { name = name, scope = scope, body = plain(body) }
        end
    end
    for index = 1, accountCount or 0 do add(index, "account") end
    for index = accountSlots + 1, accountSlots + (characterCount or 0) do add(index, "character") end
    return macros
end

ns.Snapshot.addSection("macros", collectMacros)
