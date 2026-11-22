-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section note: a short player note for the assistant (/mcp note <text>), e.g. "I want Deadmines today".
-- Lives in the character's saved dump and passes through subsequent dumps; a command without text removes it.

local _, ns = ...

local Note = {}
ns.Note = Note

local MAX_LENGTH = 500

-- The first max bytes of text without cutting a UTF-8 character in half (an invalid byte sequence would reach the dump).
function Note.clip(text, max)
    if #text <= max then return text end
    local cut = max
    while cut > 0 do
        local nextByte = text:byte(cut + 1)
        if not nextByte or nextByte < 128 or nextByte >= 192 then break end -- not a continuation byte: cut is a boundary
        cut = cut - 1
    end
    return text:sub(1, cut)
end

-- Saves the note; returns it, or nil when the text is empty (then the note disappears).
function Note.set(text)
    text = tostring(text or ""):gsub("%s+", " "):match("^%s*(.-)%s*$")
    local current = ns.Snapshot.current()
    if text == "" then
        current.note = nil
        return nil
    end
    current.note = { text = Note.clip(text, MAX_LENGTH), setAt = time() }
    return current.note
end

ns.Snapshot.addWindowSection("note", nil, nil, { quiet = true })
