-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Addon lifecycle: what we do after login and before logout.

local _, ns = ...
local Snapshot, Panel, Played, Store = ns.Snapshot, ns.Panel, ns.Played, ns.Store

local function onLogin()
    pcall(Store.adoptRealmlessDump)
    if not Store.settings().panelHidden then pcall(Panel.create) end
    pcall(ns.MinimapButton.create)
    pcall(Played.request)
    pcall(ns.Options.register)
end

-- Also fires on /reload, right before SavedVariables are written.
local function onLogout()
    pcall(Snapshot.dump, "logout")
    pcall(Snapshot.sanitizeCurrent) -- also when the dump itself failed
end

ns.Events.on("PLAYER_LOGOUT", onLogout)
ns.Events.on("PLAYER_LOGIN", onLogin)
