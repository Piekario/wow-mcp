-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Timing of dump sections (debugprofilestop, when the client has it).
-- Without a clock the recorder behaves like a plain pcall and measures nothing.

local _, ns = ...
local round1 = ns.round1

local Timing = {}
ns.Timing = Timing

local SLOWEST_COUNT = 5

-- Above this many ms /mcp status warns that the dump is heavy.
Timing.WARNING_TOTAL_MS = 200

local function readClock()
    if type(debugprofilestop) ~= "function" then return nil end
    local ok, now = pcall(debugprofilestop)
    if ok and type(now) == "number" then return now end
    return nil
end

local function slowestFirst(a, b)
    if a.ms ~= b.ms then return a.ms > b.ms end
    return a.name < b.name
end

local function summarize(laps, totalMs)
    table.sort(laps, slowestFirst)
    local slowest = {}
    for i = 1, math.min(SLOWEST_COUNT, #laps) do
        slowest[i] = { name = laps[i].name, ms = round1(laps[i].ms) }
    end
    return { totalMs = round1(totalMs), slowest = slowest }
end

-- Recorder for one dump: measure(name, fn) returns the same as pcall.
function Timing.newRecorder()
    local startedAt = readClock()
    local laps = {}
    local recorder = {}

    function recorder.measure(name, fn)
        local from = readClock()
        local ok, result = pcall(fn)
        local to = readClock()
        if from and to then laps[#laps + 1] = { name = name, ms = to - from } end
        return ok, result
    end

    -- { totalMs, slowest = { {name, ms}, ... } } or nil when there was no clock.
    function recorder.report()
        local now = readClock()
        if not (startedAt and now) then return nil end
        return summarize(laps, now - startedAt)
    end

    return recorder
end
