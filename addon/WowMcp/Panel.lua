-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Panel in game: dump age, free bag slots and a "Save" button
-- (the same as /mcp reload). Works only on a player click: when a window has given new
-- data, the panel only highlights "Save" and hints, but never saves by itself.
-- Hovering shows the list of section errors from the last dump.

local _, ns = ...
local plain, callFirst, Compat = ns.Compat.plain, ns.Compat.callFirst, ns.Compat
local Store, Snapshot, chat = ns.Store, ns.Snapshot, ns.chat

local Panel = {}
ns.Panel = Panel

local REFRESH_SECONDS = 5
local panel

local function formatAgeShort(seconds)
    local minutes = math.floor(seconds / 60)
    if minutes < 1 then return "just now" end
    if minutes < 60 then return minutes .. " min ago" end
    if minutes < 48 * 60 then return math.floor(minutes / 60) .. " h " .. (minutes % 60) .. " min ago" end
    return math.floor(minutes / 1440) .. " days ago"
end

-- Age color: green up to an hour, yellow up to two, then red (the data is already stale).
local function ageColor(seconds)
    if seconds >= 7200 then return "|cffff5555" end
    if seconds >= 3600 then return "|cffffcc00" end
    return "|cff55ff55"
end

local function ageLine(snapshot)
    if not (snapshot and type(snapshot.dumpedAt) == "number") then
        return "Dump: |cffff5555none|r"
    end
    local seconds = time() - snapshot.dumpedAt
    local line = "Dump: " .. ageColor(seconds) .. formatAgeShort(seconds) .. "|r"
    if Snapshot.isUnsaved() then line = line .. " |cffffcc00(unsaved)|r" end
    return line
end

local function freeBagSlots()
    local free = 0
    for bag = 0, NUM_BAG_SLOTS or 4 do
        local r = callFirst({ "C_Container.GetContainerNumFreeSlots", "GetContainerNumFreeSlots" }, bag)
        if r and type(plain(r[1])) == "number" then free = free + r[1] end
    end
    return free
end

local function infoLine(snapshot)
    local line = "Free slots: " .. freeBagSlots()
    if snapshot and type(snapshot.errors) == "table" and #snapshot.errors > 0 then
        line = line .. "  |cffff5555errors: " .. #snapshot.errors .. "|r"
    end
    return line
end

-- Hint shown when a window (trainer, bank, auctions...) has given data that is not on disk yet.
local function saveHint()
    if #Snapshot.pendingWindowSections() == 0 then return "" end
    return "|cffffcc00New window data: click Save|r"
end

local function panelLines()
    local snapshot = Store.savedCurrentCharacter()
    return ageLine(snapshot), infoLine(snapshot), saveHint()
end

-- Highlighting the button is only a sign for the player; only the player clicks it.
local function highlightSaveButton(on)
    local button = panel.saveButton
    if not button then return end
    if on then button:LockHighlight() else button:UnlockHighlight() end
end

function Panel.refresh()
    if not panel then return end
    local ok, age, info, hint = pcall(panelLines)
    if ok then
        if Store.settings().compact then
            -- Compact mode: one line (dump age, and an asterisk after new window data).
            panel.ageText:SetText(age .. Compat.pick(hint ~= "", " |cffffcc00*|r", ""))
            panel.infoText:SetText("")
            panel.hintText:SetText("")
        else
            panel.ageText:SetText(age)
            panel.infoText:SetText(info)
            panel.hintText:SetText(hint)
        end
        highlightSaveButton(hint ~= "")
    end
end

-- Panel tooltip: section errors from the last dump (read-only, changes nothing).
local function showErrorsTooltip(owner)
    if not GameTooltip then return end
    local snapshot = Store.savedCurrentCharacter()
    local errors = snapshot and type(snapshot.errors) == "table" and snapshot.errors or {}
    GameTooltip:SetOwner(owner, "ANCHOR_LEFT")
    if #errors == 0 then
        GameTooltip:SetText("WowMcp: no section errors")
    else
        GameTooltip:SetText("WowMcp: section errors (" .. #errors .. ")")
        for _, err in ipairs(errors) do GameTooltip:AddLine(tostring(err), 1, 0.4, 0.4, true) end
    end
    GameTooltip:Show()
end

local function hideTooltip()
    if GameTooltip then GameTooltip:Hide() end
end

local function saveAndReload(reason)
    pcall(Snapshot.dump, reason)
    ReloadUI()
end

local function placeFrame(f)
    local p = Store.settings().panelPoint
    if type(p) == "table" and type(p[1]) == "string" then
        f:SetPoint(p[1], UIParent, p[2] or p[1], tonumber(p[3]) or 0, tonumber(p[4]) or 0)
    else
        f:SetPoint("TOPRIGHT", UIParent, "TOPRIGHT", -230, -220)
    end
end

-- The player can drag the panel with the mouse; the position is kept in settings.
local function makeDraggable(f)
    f:SetClampedToScreen(true)
    f:EnableMouse(true)
    f:SetMovable(true)
    f:RegisterForDrag("LeftButton")
    f:SetScript("OnDragStart", function(self) self:StartMoving() end)
    f:SetScript("OnDragStop", function(self)
        self:StopMovingOrSizing()
        local point, _, relativePoint, x, y = self:GetPoint()
        if point then Store.settings().panelPoint = { point, relativePoint, x, y } end
    end)
end

local function addBackdrop(f)
    if not f.SetBackdrop then return end
    f:SetBackdrop({
        bgFile = "Interface\\Tooltips\\UI-Tooltip-Background",
        edgeFile = "Interface\\Tooltips\\UI-Tooltip-Border",
        tile = true, tileSize = 16, edgeSize = 12,
        insets = { left = 3, right = 3, top = 3, bottom = 3 },
    })
    f:SetBackdropColor(0, 0, 0, 0.8)
end

local function addTexts(f)
    local title = f:CreateFontString(nil, "OVERLAY", "GameFontNormalSmall")
    title:SetPoint("TOPLEFT", f, "TOPLEFT", 10, -9)
    title:SetText("WowMcp")
    f.titleText = title
    f.ageText = f:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
    f.ageText:SetPoint("TOPLEFT", title, "BOTTOMLEFT", 0, -5)
    f.infoText = f:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
    f.infoText:SetPoint("TOPLEFT", f.ageText, "BOTTOMLEFT", 0, -3)
    f.hintText = f:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
    f.hintText:SetPoint("TOPLEFT", f.infoText, "BOTTOMLEFT", 0, -3)
end

local function addSaveButton(f)
    local save = CreateFrame("Button", nil, f, "UIPanelButtonTemplate")
    save:SetSize(80, 20)
    save:SetPoint("BOTTOMRIGHT", f, "BOTTOMRIGHT", -8, 8)
    save:SetText("Save")
    save:SetScript("OnClick", function() saveAndReload("panel") end)
    save:SetScript("OnEnter", function(self)
        if GameTooltip then
            GameTooltip:SetOwner(self, "ANCHOR_TOP")
            GameTooltip:SetText("Dump character state and /reload\n(same as /mcp reload)")
            GameTooltip:Show()
        end
    end)
    save:SetScript("OnLeave", hideTooltip)
    f.saveButton = save
end

local function addCloseButton(f)
    local close = CreateFrame("Button", nil, f, "UIPanelCloseButton")
    close:SetSize(22, 22)
    close:SetPoint("TOPRIGHT", f, "TOPRIGHT", -1, -1)
    close:SetScript("OnClick", function()
        f:Hide()
        Store.settings().panelHidden = true
        chat("Panel hidden. /mcp panel shows it again.")
    end)
    f.closeButton = close
end

-- Panel layout: normal (four lines, button in the corner) or compact (one line and a Save button).
-- Changes only the look; the Save button does the same and still only on a player click.
local function setVisible(region, visible)
    if visible then region:Show() else region:Hide() end
end

local function applyLayout(f)
    local compact = Store.settings().compact and true or false
    for _, region in ipairs({ f.titleText, f.infoText, f.hintText, f.closeButton }) do
        setVisible(region, not compact)
    end
    f.ageText:ClearAllPoints()
    f.saveButton:ClearAllPoints()
    if compact then
        f:SetSize(230, 28)
        f.ageText:SetPoint("LEFT", f, "LEFT", 8, 0)
        f.saveButton:SetSize(64, 20)
        f.saveButton:SetPoint("RIGHT", f, "RIGHT", -4, 0)
    else
        f:SetSize(190, 100)
        f.ageText:SetPoint("TOPLEFT", f.titleText, "BOTTOMLEFT", 0, -5)
        f.saveButton:SetSize(80, 20)
        f.saveButton:SetPoint("BOTTOMRIGHT", f, "BOTTOMRIGHT", -8, 8)
    end
end

function Panel.isCompact()
    return Store.settings().compact and true or false
end

-- /mcp compact on|off: remembers the choice and switches the panel (if it already exists).
function Panel.setCompact(enabled)
    Store.settings().compact = Compat.pick(enabled, true, nil)
    if panel then
        pcall(applyLayout, panel)
        Panel.refresh()
    end
end

-- Refreshes the text every few seconds; only reads the time and counts free slots.
local function refreshPeriodically(f)
    local elapsed = 0
    f:SetScript("OnUpdate", function(_, delta)
        elapsed = elapsed + (delta or 0)
        if elapsed >= REFRESH_SECONDS then
            elapsed = 0
            Panel.refresh()
        end
    end)
    f:SetScript("OnShow", Panel.refresh)
end

local function addErrorsTooltip(f)
    f:SetScript("OnEnter", showErrorsTooltip)
    f:SetScript("OnLeave", hideTooltip)
end

-- Creates the panel (once); returns the frame, or nil when the client lacks CreateFrame/UIParent.
function Panel.create()
    if panel then return panel end
    if not (CreateFrame and UIParent) then return nil end
    local f = CreateFrame("Frame", "WowMcpPanel", UIParent, BackdropTemplateMixin and "BackdropTemplate" or nil)
    f:SetSize(190, 100)
    placeFrame(f)
    f:SetFrameStrata("MEDIUM")
    makeDraggable(f)
    addBackdrop(f)
    addTexts(f)
    addSaveButton(f)
    addCloseButton(f)
    if Store.settings().compact then pcall(applyLayout, f) end
    addErrorsTooltip(f)
    refreshPeriodically(f)
    panel = f
    Panel.refresh()
    return f
end

-- New window data refreshes the panel immediately, without waiting for the timer.
Snapshot.onWindowCaptured(Panel.refresh)

function Panel.toggle()
    local ok, f = pcall(Panel.create)
    if not ok or not f then
        chat("Could not create the panel: " .. tostring(f))
        return
    end
    local settings = Store.settings()
    if settings.panelHidden then
        settings.panelHidden = nil
        f:Show()
        chat("Panel visible. You can drag it with the mouse.")
    else
        settings.panelHidden = true
        f:Hide()
        chat("Panel hidden. /mcp panel shows it again.")
    end
end
