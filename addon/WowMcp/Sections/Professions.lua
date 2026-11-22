-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Section professions: professions and other skills from the Skills window, together with
-- weapon skills (group = "weapon").

local _, ns = ...
local plain = ns.Compat.plain

-- Weapon skills: skill line IDs have been constant since the classic version of the game.
-- The client returns nil for a skill the character does not know.
local WEAPON_SKILLS = {
    { id = 43, name = "Swords" }, { id = 55, name = "Two-Handed Swords" },
    { id = 44, name = "Axes" }, { id = 172, name = "Two-Handed Axes" },
    { id = 54, name = "Maces" }, { id = 160, name = "Two-Handed Maces" },
    { id = 173, name = "Daggers" }, { id = 136, name = "Staves" },
    { id = 229, name = "Polearms" }, { id = 473, name = "Fist Weapons" },
    { id = 45, name = "Bows" }, { id = 46, name = "Guns" },
    { id = 226, name = "Crossbows" }, { id = 176, name = "Thrown" },
    { id = 162, name = "Unarmed" }, { id = 95, name = "Defense" },
}

-- Profession skill line IDs (constant since the classic game): the dump gets skillID next to the name, which
-- is in the client's language, so the server can recognise a profession on any client.
local PROFESSION_SKILLS = { 129, 164, 165, 171, 182, 185, 186, 197, 202, 333, 356, 393 }

-- Name of a skill line in the client's language: C_SkillInfo.GetSkillLineInfoByID returns a table or several values.
local function skillLineName(id)
    if not (C_SkillInfo and C_SkillInfo.GetSkillLineInfoByID) then return nil end
    local ok, a = pcall(C_SkillInfo.GetSkillLineInfoByID, id)
    if not ok then return nil end
    if type(a) == "table" then return plain(a.name) or plain(a.skillLineDisplayName) end
    return plain(a)
end

-- Adds skillID to profession entries that came without one (the old Skills window has no IDs): the client's
-- own name of each profession skill line is matched against the entry name.
local function addProfessionSkillIDs(professions)
    local byName = {}
    for _, id in ipairs(PROFESSION_SKILLS) do
        local name = skillLineName(id)
        if type(name) == "string" and name ~= "" then byName[name] = id end
    end
    for _, entry in ipairs(professions) do
        if entry.skillID == nil and entry.name then entry.skillID = byName[entry.name] end
    end
end

-- Weapon skill entry from C_SkillInfo.GetSkillLineInfoByID (a table or multiple values).
local function weaponSkillEntry(skill, a, b, c)
    local entry = { name = skill.name, group = "weapon", skillID = skill.id }
    if type(a) == "table" then
        -- Table variant: fields name, rank, maxRank.
        entry.name = plain(a.name) or plain(a.skillLineDisplayName) or skill.name
        entry.rank = plain(a.rank) or plain(a.skillLevel)
        entry.maxRank = plain(a.maxRank) or plain(a.maxSkillLevel)
    else
        -- Multi-value variant: name, rank, maxRank.
        if type(a) == "string" then entry.name = a end
        entry.rank = plain(b)
        entry.maxRank = plain(c)
    end
    return entry
end

local function appendWeaponSkills(professions)
    if not (C_SkillInfo and C_SkillInfo.GetSkillLineInfoByID) then return end
    -- The old skill list already contains weapons; we do not duplicate entries.
    local known = {}
    for _, entry in ipairs(professions) do known[entry.name or ""] = true end
    for _, skill in ipairs(WEAPON_SKILLS) do
        local ok, a, b, c = pcall(C_SkillInfo.GetSkillLineInfoByID, skill.id)
        if ok and a ~= nil then
            local entry = weaponSkillEntry(skill, a, b, c)
            if entry.rank and not known[entry.name] then
                professions[#professions + 1] = entry
            end
        end
    end
end

-- Old Skills window: all skills with group headers.
local function skillLines()
    local professions = {}
    local header
    for i = 1, GetNumSkillLines() or 0 do
        local name, isHeader, _, rank, _, _, maxRank = GetSkillLineInfo(i)
        if isHeader then
            header = name
        elseif name then
            professions[#professions + 1] = { name = name, rank = rank, maxRank = maxRank, group = header }
        end
    end
    return professions
end

local MAX_PROFESSION_SLOTS = 6

-- New API: professions only (primary, secondary), no headers.
local function professionSlots()
    local professions = {}
    local indexes = { GetProfessions() }
    for i = 1, MAX_PROFESSION_SLOTS do
        local index = indexes[i]
        if index then
            local name, _, rank, maxRank, _, _, skillLine = GetProfessionInfo(index)
            if name then
                local entry = { name = name, rank = rank, maxRank = maxRank }
                if type(plain(skillLine)) == "number" then entry.skillID = plain(skillLine) end
                professions[#professions + 1] = entry
            end
        end
    end
    return professions
end

local function collectProfessions()
    local professions = {}
    if GetNumSkillLines and GetSkillLineInfo then
        professions = skillLines()
    elseif GetProfessions and GetProfessionInfo then
        professions = professionSlots()
    end
    addProfessionSkillIDs(professions)
    appendWeaponSkills(professions)
    return professions
end

ns.Snapshot.addSection("professions", collectProfessions)
