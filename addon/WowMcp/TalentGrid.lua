-- SPDX-License-Identifier: GPL-3.0-or-later
-- Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

-- Talent tree grid. C_Traits gives node positions in pixels (in Forever every 600:
-- rows 2130, 2730, ...), sometimes with jitter (5740 instead of 5730) or 10 times too large
-- (39300 instead of 3930). TalentGrid converts them to row and column numbers starting at 1 within
-- a tab. Pure functions only (no game API), tested in addon/tests.

local _, ns = ...

local TalentGrid = {}
ns.TalentGrid = TalentGrid

TalentGrid.JITTER_SHARE = 0.02 -- difference up to 2% of the value is jitter, not another row
TalentGrid.SCALE_ERROR = 10    -- a position this many times too large is a data error in the client
TalentGrid.SCALE_OUTLIER = 5   -- ...but only when it is over 5 times larger than the median

local function sortedNumbers(values)
    local out = {}
    for _, v in pairs(values) do
        if type(v) == "number" then out[#out + 1] = v end
    end
    table.sort(out)
    return out
end

local function median(sorted)
    return sorted[math.ceil(#sorted / 2)]
end

local function jitterOf(value)
    return math.abs(value) * TalentGrid.JITTER_SHARE
end

-- Whether the value lies (with jitter) near one of the other values.
local function nearAny(value, sorted)
    for _, other in ipairs(sorted) do
        if math.abs(value - other) <= jitterOf(other) then return true end
    end
    return false
end

-- Values without suspiciously large ones (over SCALE_OUTLIER times the median), ascending.
function TalentGrid.regular(values)
    local sorted = sortedNumbers(values)
    local out = {}
    if #sorted == 0 then return out end
    local limit = math.abs(median(sorted)) * TalentGrid.SCALE_OUTLIER
    for _, v in ipairs(sorted) do
        if math.abs(v) <= limit then out[#out + 1] = v end
    end
    return out
end

-- Whether the value lies (with jitter) on a grid node with origin origin and step step.
local function onGrid(value, origin, step)
    if not (origin and step) then return false end
    local offset = (value - origin) / step
    return offset > -0.5 and math.abs(offset - math.floor(offset + 0.5)) * step <= jitterOf(value)
end

-- Values with corrected positions that were 10x too large (same keys). A position is 10x too large
-- when, after dividing, it lands on another position of the tab or (with a known step) on a grid node.
function TalentGrid.unscale(values, step)
    local regular = TalentGrid.regular(values)
    local limit = regular[#regular]
    local out = {}
    for key, v in pairs(values) do
        out[key] = v
        if type(v) == "number" and limit and v > limit then
            local candidate = v / TalentGrid.SCALE_ERROR
            if nearAny(candidate, regular) or onGrid(candidate, regular[1], step) then out[key] = candidate end
        end
    end
    return out
end

-- Clusters of close values (jitter within one row): list of means, ascending.
function TalentGrid.clusters(values)
    local sorted = sortedNumbers(values)
    local centers, sum, count = {}, 0, 0
    for i, v in ipairs(sorted) do
        if count > 0 and v - sorted[i - 1] > jitterOf(v) then
            centers[#centers + 1] = sum / count
            sum, count = 0, 0
        end
        sum, count = sum + v, count + 1
    end
    if count > 0 then centers[#centers + 1] = sum / count end
    return centers
end

-- Grid step: the smallest gap between neighbouring clusters; nil for a single cluster.
function TalentGrid.step(centers)
    local step
    for i = 2, #centers do
        local gap = centers[i] - centers[i - 1]
        if not step or gap < step then step = gap end
    end
    return step
end

-- Grid step from the values of one tab (without suspiciously large positions).
function TalentGrid.stepOf(values)
    return TalentGrid.step(TalentGrid.clusters(TalentGrid.regular(values)))
end

-- Grid numbers from 1 for the values of one tab (same keys). An empty row in the middle
-- keeps its number. step: the step shared by the tree; without it computed from these values.
function TalentGrid.indexes(values, step)
    step = step or TalentGrid.stepOf(values)
    local unscaled = TalentGrid.unscale(values, step)
    local origin = TalentGrid.clusters(unscaled)[1]
    local out = {}
    for key, v in pairs(unscaled) do
        if type(v) == "number" then
            out[key] = step and (1 + math.floor((v - origin) / step + 0.5)) or 1
        end
    end
    return out
end

-- Shared step for several tabs: the median of the tabs' steps (a tab with gaps in rows
-- or with a single node does not spoil the result).
function TalentGrid.sharedStep(valueLists)
    local steps = {}
    for _, values in ipairs(valueLists) do
        local step = TalentGrid.stepOf(values)
        if step then steps[#steps + 1] = step end
    end
    if #steps == 0 then return nil end
    table.sort(steps)
    return median(steps)
end

-- Nodes grouped by tab, in order of first occurrence.
local function nodesByTree(nodes)
    local trees, byName = {}, {}
    for _, node in ipairs(nodes) do
        local name = node.tree or ""
        if not byName[name] then
            byName[name] = {}
            trees[#trees + 1] = byName[name]
        end
        table.insert(byName[name], node)
    end
    return trees
end

local function positions(treeNodes, field)
    local values = {}
    for i, node in ipairs(treeNodes) do values[i] = node[field] end
    return values
end

-- Sets row (from posY) and column (from posX) on nodes; numbers computed separately in each tab.
-- The grid step is shared by the whole tree and both axes (the C_Traits grid is square:
-- in the real dump 600 horizontally and vertically), so a tab with a few nodes also gets a good step.
function TalentGrid.place(nodes)
    local trees = nodesByTree(nodes)
    local xs, ys, all = {}, {}, {}
    for i, treeNodes in ipairs(trees) do
        xs[i], ys[i] = positions(treeNodes, "posX"), positions(treeNodes, "posY")
        all[#all + 1], all[#all + 2] = xs[i], ys[i]
    end
    local step = TalentGrid.sharedStep(all)
    for i, treeNodes in ipairs(trees) do
        local columns, rows = TalentGrid.indexes(xs[i], step), TalentGrid.indexes(ys[i], step)
        for j, node in ipairs(treeNodes) do node.row, node.column = rows[j], columns[j] end
    end
end
