-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- SavedVariables (WowMcpDB): character dumps, settings and the beasts observed on the account.
-- The game replaces WowMcpDB after the addon files load, so every
-- function reads the global at call time and never caches it.

local _, ns = ...

WowMcpDB = WowMcpDB or {}

local Store = {}
ns.Store = Store

local sessionKey

local function realmName()
    for _, name in ipairs({ "GetRealmName", "GetNormalizedRealmName" }) do
        local read = _G[name]
        if type(read) == "function" then
            local ok, realm = pcall(read)
            if ok and type(realm) == "string" and realm ~= "" then return realm end
        end
    end
    return nil
end

-- The player's GUID is "Player-<realm number>-<id>" also in builds that give no realm name, so two characters with
-- one name on two realms still get two keys.
function Store.realmId()
    local read = _G.UnitGUID
    if type(read) ~= "function" then return nil end
    local ok, guid = pcall(read, "player")
    guid = ok and ns.Compat.plain(guid)
    return type(guid) == "string" and guid:match("^Player%-(%d+)%-") or nil
end

local function playerTokens()
    local _, classToken = UnitClass("player")
    local _, raceToken = UnitRace("player")
    return classToken, raceToken
end

-- Some client builds return no realm name (seen in Forever build 71037). The character must then keep the
-- key it already has, or its history splits and a character with the same name on another realm overwrites
-- it. Returns the one saved "Name-Realm" key whose dump has the player's class and race, or nil when there is
-- none or more than one.
local function savedRealmKey(name)
    local characters = WowMcpDB.characters
    if type(characters) ~= "table" then return nil end
    local classToken, raceToken = playerTokens()
    if not classToken then return nil end
    local prefix, found = name .. "-", nil
    for key, dump in pairs(characters) do
        local character = type(dump) == "table" and dump.character
        if type(key) == "string" and key:sub(1, #prefix) == prefix and type(character) == "table"
            and character.classToken == classToken and character.raceToken == raceToken then
            if found then return nil end
            found = key
        end
    end
    return found
end

-- Character key in WowMcpDB.characters: "Name-Realm"; when the client gives no realm name, the saved
-- key of this character (savedRealmKey), then "Name-<realm number>" from the GUID, and only then just the name.
function Store.characterKey()
    if sessionKey then return sessionKey end
    local name = UnitName("player") or "unknown"
    local realm = realmName()
    local key = realm and (name .. "-" .. realm) or savedRealmKey(name)
    local realmId = not key and Store.realmId()
    if realmId then key = name .. "-" .. realmId end
    -- Remembered for the session: at logout the client may no longer tell the class, and the key must not change.
    sessionKey = key
    return key or name
end

-- Dumps saved under the bare name while the client gave no realm name belong to the "Name-Realm" key: the
-- newer of the two dumps stays under that key and the bare entry is removed. Does nothing for another
-- character with the same name (class or race differs). Returns the key that received the dump, or nil.
function Store.adoptRealmlessDump()
    local characters = WowMcpDB.characters
    local name = UnitName("player")
    if type(characters) ~= "table" or type(name) ~= "string" then return nil end
    local key, bare = Store.characterKey(), characters[name]
    if key == name or type(bare) ~= "table" then return nil end
    local classToken, raceToken = playerTokens()
    local character = bare.character
    if type(character) ~= "table" or character.classToken ~= classToken or character.raceToken ~= raceToken then
        return nil
    end
    local kept = characters[key]
    local bareAt = type(bare.dumpedAt) == "number" and bare.dumpedAt or 0
    local keptAt = type(kept) == "table" and type(kept.dumpedAt) == "number" and kept.dumpedAt or 0
    if type(kept) ~= "table" or bareAt > keptAt then characters[key] = bare end
    characters[name] = nil
    if WowMcpDB.last == name then WowMcpDB.last = key end
    return key
end

-- Dump of the character being played; creates an empty one if none exists yet.
function Store.currentCharacter()
    local key = Store.characterKey()
    WowMcpDB.characters = WowMcpDB.characters or {}
    WowMcpDB.characters[key] = WowMcpDB.characters[key] or {}
    return WowMcpDB.characters[key]
end

-- Prepares WowMcpDB for a new dump; returns the character key -> dump map.
function Store.openCharacters()
    WowMcpDB.schema = ns.SCHEMA_VERSION
    WowMcpDB.characters = WowMcpDB.characters or {}
    return WowMcpDB.characters
end

function Store.saveCharacter(key, snapshot)
    WowMcpDB.characters[key] = snapshot
    WowMcpDB.last = key
end

-- Saved dump of the character being played, or nil; creates nothing.
function Store.savedCurrentCharacter()
    return WowMcpDB.characters and WowMcpDB.characters[Store.characterKey()]
end

-- Key and dump of the character from the last dump on this account.
function Store.lastSaved()
    local key = WowMcpDB.last
    return key, key and WowMcpDB.characters and WowMcpDB.characters[key]
end

-- Settings for the panel, played time and disabled sections (panelHidden, panelPoint, played, disabledSections).
function Store.settings()
    if type(WowMcpDB.settings) ~= "table" then WowMcpDB.settings = {} end
    return WowMcpDB.settings
end

-- Whether the section is disabled by /mcp off. Creates nothing in settings.
function Store.isSectionDisabled(name)
    local disabled = Store.settings().disabledSections
    return type(disabled) == "table" and disabled[name] == true
end

function Store.setSectionDisabled(name, disabled)
    local settings = Store.settings()
    if disabled then
        if type(settings.disabledSections) ~= "table" then settings.disabledSections = {} end
        settings.disabledSections[name] = true
    elseif type(settings.disabledSections) == "table" then
        settings.disabledSections[name] = nil
        if next(settings.disabledSections) == nil then settings.disabledSections = nil end
    end
end

-- Beasts the account's Hunters tamed or examined with Beast Lore (Beasts.lua): creature ID -> observation. Account-wide,
-- because a beast is the same for every character; each Hunter's dump carries a copy (section beasts).
function Store.beasts()
    if type(WowMcpDB.beasts) ~= "table" then WowMcpDB.beasts = {} end
    return WowMcpDB.beasts
end
