-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section spells (spellbook). Reading a spellbook entry is shared with the pet's
-- spellbook (Pet.lua), and the spell list with cooldowns (Cooldowns.lua).

local _, ns = ...

local Compat = ns.Compat
local Spellbook = {}
ns.Spellbook = Spellbook

-- Spellbook entry from C_SpellBook.GetSpellBookItemInfo. petAction marks pet commands and stances (the entry type
-- Enum.SpellBookItemType.PetAction), so the server does not depend on the subtext ("Pet Stance") in the client's language.
function Spellbook.entryFromItemInfo(info)
    local petActionType = Enum and Enum.SpellBookItemType and Enum.SpellBookItemType.PetAction
    return {
        id = info.spellID or info.actionID,
        name = info.name,
        rank = (info.subName and info.subName ~= "") and info.subName or nil,
        passive = info.isPassive and true or nil,
        icon = Compat.plain(info.iconID),
        petAction = (petActionType ~= nil and info.itemType == petActionType) or nil,
    }
end

-- true when the client says the spell is a buff the character casts on itself (C_Spell.IsSelfBuff); nil otherwise.
-- The server compares the spell's icon with the icons of active auras, so it needs no spell names to tell
-- that a self buff is not active.
local function selfBuffFlag(spell)
    if not (spell.id and not spell.passive and C_Spell and C_Spell.IsSelfBuff) then return nil end
    local ok, result = pcall(C_Spell.IsSelfBuff, spell.id)
    if ok and Compat.plain(result) == true then return true end
    return nil
end

local function modernSpellbook()
    local spells = {}
    local bank = Enum and Enum.SpellBookSpellBank and Enum.SpellBookSpellBank.Player or 0
    for line = 1, C_SpellBook.GetNumSpellBookSkillLines() or 0 do
        local lineInfo = C_SpellBook.GetSpellBookSkillLineInfo(line)
        if lineInfo then
            local offset = lineInfo.itemIndexOffset or 0
            for i = offset + 1, offset + (lineInfo.numSpellBookItems or 0) do
                local info = C_SpellBook.GetSpellBookItemInfo and C_SpellBook.GetSpellBookItemInfo(i, bank)
                if info and info.name then
                    local spell = Spellbook.entryFromItemInfo(info)
                    spell.tab = lineInfo.name
                    spell.selfBuff = selfBuffFlag(spell)
                    spells[#spells + 1] = spell
                end
            end
        end
    end
    return spells
end

local function legacySpellbook()
    local spells = {}
    for tab = 1, GetNumSpellTabs() or 0 do
        local tabName, _, offset, numSpells = GetSpellTabInfo(tab)
        for i = (offset or 0) + 1, (offset or 0) + (numSpells or 0) do
            local name, rank = GetSpellBookItemName(i, "spell")
            if name then
                spells[#spells + 1] = {
                    name = name,
                    rank = (rank and rank ~= "") and rank or nil,
                    tab = tabName,
                }
            end
        end
    end
    return spells
end

-- All of the character's spells with the spellbook tab name.
function Spellbook.knownSpells()
    if C_SpellBook and C_SpellBook.GetNumSpellBookSkillLines and C_SpellBook.GetSpellBookSkillLineInfo then
        return modernSpellbook()
    end
    if GetNumSpellTabs and GetSpellTabInfo and GetSpellBookItemName then
        return legacySpellbook()
    end
    return {}
end

ns.Snapshot.addSection("spells", Spellbook.knownSpells)
