-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Standing class buffs of which one is enough (all Aspects, all Blessings...), as one spell ID per spell. The client
-- answers with the spell's name in its own language, and every rank of a spell has the same name, so the server can
-- group spells and auras on any client without a table of names. The IDs come from the original game and are not
-- verified for Forever: the server compares the answers with its English rules and reports an ID that does not fit.

local _, ns = ...

local ClassBuffs = {}
ns.ClassBuffs = ClassBuffs

local GROUPS = {
    HUNTER = {
        { label = "Aspect", ids = { 13163, 13165, 5118, 13159, 13161, 20043 } },
    },
    MAGE = {
        { label = "Arcane Intellect", ids = { 1459 } },
        { label = "Armor", ids = { 168, 7302, 6117, 30482 } },
    },
    PRIEST = {
        { label = "Power Word: Fortitude", ids = { 1243 } },
    },
    DRUID = {
        { label = "Mark of the Wild", ids = { 1126 } },
    },
    WARRIOR = {
        { label = "Battle Shout", ids = { 6673 } },
    },
    PALADIN = {
        { label = "Blessing", ids = { 19740, 19742, 20217, 1038, 19977, 20911 } },
    },
    WARLOCK = {
        { label = "Armor", ids = { 687, 706, 28176 } },
    },
    SHAMAN = {
        { label = "Shield", ids = { 324, 24398 } },
    },
}

-- The groups of the class token, or nil when the class has none.
function ClassBuffs.groupsFor(classToken)
    return GROUPS[classToken]
end
