-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- The sharing window (/mcp share): the client report as text to copy, and the address of the issue form to paste
-- it into. It only shows text: the addon sends nothing anywhere, and the game gives it no way to.

local _, ns = ...
local ShareReport, chat = ns.ShareReport, ns.chat

local ShareWindow = {}
ns.ShareWindow = ShareWindow

local FRAME_NAME = "WowMcpShareWindow"
local WIDTH, HEIGHT, MARGIN = 520, 380, 14
local INTRO = "Help WowMcp support your class, client and language. The report below holds no names, items or gold."
    .. " Press Ctrl+C to copy it, then paste it into a new issue at the address underneath."
    .. " For the full anonymized dump, ask your assistant to \"share an anonymized dump\" instead."

local window
local opened = {} -- callbacks run each time the window is shown

function ShareWindow.onOpen(callback)
    opened[#opened + 1] = callback
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

-- A box whose text can be selected and copied but not changed: typing puts the text back.
local function makeCopyOnly(box)
    box:SetAutoFocus(false)
    box:SetScript("OnTextChanged", function(self, userInput)
        if userInput and self.fixedText then self:SetText(self.fixedText) end
    end)
    box:SetScript("OnEditFocusGained", function(self) self:HighlightText() end)
    box:SetScript("OnEscapePressed", function(self) self:ClearFocus() end)
end

local function setFixedText(box, text)
    box.fixedText = text
    box:SetText(text)
end

local function addReportBox(f)
    local scroll = CreateFrame("ScrollFrame", FRAME_NAME .. "Scroll", f, "UIPanelScrollFrameTemplate")
    scroll:SetPoint("TOPLEFT", f, "TOPLEFT", MARGIN, -86)
    scroll:SetPoint("BOTTOMRIGHT", f, "BOTTOMRIGHT", -MARGIN - 22, 78)
    local box = CreateFrame("EditBox", nil, scroll)
    box:SetMultiLine(true)
    box:SetFontObject("ChatFontNormal")
    box:SetWidth(WIDTH - 2 * MARGIN - 26)
    makeCopyOnly(box)
    scroll:SetScrollChild(box)
    f.reportBox = box
end

local function addAddressBox(f)
    local label = f:CreateFontString(nil, "OVERLAY", "GameFontNormalSmall")
    label:SetPoint("BOTTOMLEFT", f, "BOTTOMLEFT", MARGIN, 60)
    label:SetText("Paste the report here (copy the address into your browser):")
    local box = CreateFrame("EditBox", nil, f, "InputBoxTemplate")
    box:SetSize(WIDTH - 2 * MARGIN - 8, 20)
    box:SetPoint("BOTTOMLEFT", f, "BOTTOMLEFT", MARGIN + 6, 38)
    makeCopyOnly(box)
    f.addressBox = box
end

local function addButtons(f)
    local close = CreateFrame("Button", nil, f, "UIPanelButtonTemplate")
    close:SetSize(90, 22)
    close:SetPoint("BOTTOMRIGHT", f, "BOTTOMRIGHT", -MARGIN, 10)
    close:SetText("Close")
    close:SetScript("OnClick", function() f:Hide() end)
    local quiet = CreateFrame("Button", nil, f, "UIPanelButtonTemplate")
    quiet:SetSize(170, 22)
    quiet:SetPoint("BOTTOMLEFT", f, "BOTTOMLEFT", MARGIN, 10)
    quiet:SetText("Do not remind me")
    quiet:SetScript("OnClick", function()
        ns.ShareReminder.setEnabled(false)
        chat("No more sharing reminders. The window stays available: /mcp share. Turn reminders on: /mcp share on")
        f:Hide()
    end)
    f.quietButton = quiet
end

local function addTexts(f)
    local title = f:CreateFontString(nil, "OVERLAY", "GameFontNormalLarge")
    title:SetPoint("TOPLEFT", f, "TOPLEFT", MARGIN, -12)
    title:SetText("WowMcp: share a client report")
    local intro = f:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
    intro:SetPoint("TOPLEFT", title, "BOTTOMLEFT", 0, -8)
    intro:SetWidth(WIDTH - 2 * MARGIN)
    intro:SetJustifyH("LEFT")
    intro:SetText(INTRO)
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
    addTexts(f)
    addReportBox(f)
    addAddressBox(f)
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
    setFixedText(window.reportBox, ShareReport.text())
    setFixedText(window.addressBox, ShareReport.ISSUE_URL)
    window:Show()
    for _, callback in ipairs(opened) do pcall(callback) end
    return true
end

function ShareWindow.isShown()
    return window ~= nil and window:IsShown() and true or false
end
