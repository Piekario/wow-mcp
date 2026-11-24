-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- The sharing window (/mcp share), laid out like QuestieTrace's export window: the address of the issue form, the
-- client report as one line of text to copy, a few counters and two buttons. It only shows text: the addon sends
-- nothing anywhere, and the game gives it no way to.

local _, ns = ...
local ShareReport, chat = ns.ShareReport, ns.chat

local ShareWindow = {}
ns.ShareWindow = ShareWindow

local FRAME_NAME = "WowMcpShareWindow"
local WIDTH, HEIGHT, MARGIN = 560, 250, 16
local INTRO = "Copy the text below and paste it into a new issue at the address above. It tells the project how the addon"
    .. " works for your class, client and language. Character, realm, guild and player names are never included,"
    .. " and neither are items or gold. Nothing is sent from the game. Reminder off: /mcp share off"
local NO_DUMP = "No dump yet: type /mcp and open this window again."

local window
local reported = {} -- callbacks run when the player clicks "I reported this"

function ShareWindow.onReported(callback)
    reported[#reported + 1] = callback
end

local function addBackdrop(f)
    if not f.SetBackdrop then return end
    f:SetBackdrop({
        bgFile = "Interface\\Tooltips\\UI-Tooltip-Background",
        edgeFile = "Interface\\Tooltips\\UI-Tooltip-Border",
        tile = true, tileSize = 16, edgeSize = 12,
        insets = { left = 3, right = 3, top = 3, bottom = 3 },
    })
    f:SetBackdropColor(0, 0, 0, 0.9)
end

-- A one-line box whose text can be selected and copied but not changed: typing puts the text back.
local function addCopyBox(f, y)
    local box = CreateFrame("EditBox", nil, f, "InputBoxTemplate")
    box:SetSize(WIDTH - 2 * MARGIN - 8, 20)
    box:SetPoint("TOPLEFT", f, "TOPLEFT", MARGIN + 6, y)
    box:SetAutoFocus(false)
    box:SetScript("OnTextChanged", function(self, userInput)
        if userInput and self.fixedText then
            self:SetText(self.fixedText)
            self:HighlightText()
        end
    end)
    box:SetScript("OnEditFocusGained", function(self) self:HighlightText() end)
    box:SetScript("OnEscapePressed", function(self) self:ClearFocus() end)
    return box
end

local function setFixedText(box, text)
    box.fixedText = text
    box:SetText(text)
    box:SetCursorPosition(0)
end

local function addText(f, template, y, text)
    local line = f:CreateFontString(nil, "OVERLAY", template)
    line:SetPoint("TOPLEFT", f, "TOPLEFT", MARGIN, y)
    line:SetWidth(WIDTH - 2 * MARGIN)
    line:SetJustifyH("LEFT")
    line:SetText(text or "")
    return line
end

local function addButtons(f)
    local close = CreateFrame("Button", nil, f, "UIPanelButtonTemplate")
    close:SetSize(110, 22)
    close:SetPoint("BOTTOMRIGHT", f, "BOTTOMRIGHT", -MARGIN, 12)
    close:SetText("Close")
    close:SetScript("OnClick", function() f:Hide() end)
    local done = CreateFrame("Button", nil, f, "UIPanelButtonTemplate")
    done:SetSize(150, 22)
    done:SetPoint("RIGHT", close, "LEFT", -12, 0)
    done:SetText("I reported this")
    done:SetScript("OnClick", function()
        for _, callback in ipairs(reported) do pcall(callback) end
        chat("Thank you. You will not be asked again for this class, client and language.")
        f:Hide()
    end)
    f.reportedButton = done
end

local function countersText()
    local c = ShareReport.counters()
    if not c then return NO_DUMP end
    local dump = c.dumpMs and (c.dumpMs .. " ms") or "?"
    local api = c.apiFunctions > 0 and tostring(c.apiFunctions) or "none (type /mcp probe to add them)"
    return ("Sections in the dump: |cffffd100%d|r    Addon errors: |cffffd100%d|r    Dump time: |cffffd100%s|r\nClient API functions listed: |cffffd100%s|r")
        :format(c.sections, c.errors, dump, api)
end

local function closeOnEscape()
    local special = _G.UISpecialFrames
    if type(special) == "table" then special[#special + 1] = FRAME_NAME end
end

local function create()
    local f = CreateFrame("Frame", FRAME_NAME, UIParent, BackdropTemplateMixin and "BackdropTemplate" or nil)
    f:SetSize(WIDTH, HEIGHT)
    f:SetPoint("CENTER", UIParent, "CENTER", 0, 40)
    f:SetFrameStrata("DIALOG")
    f:EnableMouse(true)
    f:SetMovable(true)
    f:SetClampedToScreen(true)
    f:RegisterForDrag("LeftButton")
    f:SetScript("OnDragStart", function(self) self:StartMoving() end)
    f:SetScript("OnDragStop", function(self) self:StopMovingOrSizing() end)
    addBackdrop(f)
    addText(f, "GameFontNormal", -12, "WowMcp: share a client report")
    f.addressBox = addCopyBox(f, -34)
    addText(f, "GameFontHighlightSmall", -62, INTRO)
    f.reportBox = addCopyBox(f, -112)
    f.countersText = addText(f, "GameFontHighlight", -144)
    addButtons(f)
    closeOnEscape()
    f:Hide()
    return f
end

-- Shows the window with a fresh report. Returns false when the client could not build it; the report address
-- is then printed in chat.
function ShareWindow.open()
    if not window then
        local ok, frame = pcall(create)
        if ok then window = frame end
    end
    if not window then
        chat("Could not open the sharing window in this client. The form is at " .. ShareReport.ISSUE_URL)
        return false
    end
    setFixedText(window.addressBox, ShareReport.ISSUE_URL)
    setFixedText(window.reportBox, ShareReport.text())
    window.countersText:SetText(countersText())
    window:Show()
    return true
end

function ShareWindow.isShown()
    return window ~= nil and window:IsShown() and true or false
end
