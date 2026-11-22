// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DefaultMaxResponseKB is the default answer size limit (flag --max-response-kb).
const DefaultMaxResponseKB = 48

// Answers over the limit are not cut blindly. The model gets a compact envelope instead: the size, how to ask for less
// (from the tool's own arguments) and a deterministic digest of what the answer holds, so the next call can be precise.

// partialSteps are the numbers of array items kept per level, from the most generous to none; the first digest that fits wins.
var partialSteps = []int{20, 10, 5, 3, 1, 0}

const (
	maxTopLevelEntries = 60   // entries listed in topLevel before the rest is summarized
	maxDigestDepth     = 3    // nesting kept in the partial answer; deeper containers become a one-line summary
	maxDigestString    = 400  // longest string kept whole in the partial answer
	bigObjectBytes     = 4096 // a top-level object this big is also listed key by key in topLevel
)

// truncation is the envelope returned instead of an answer that is over the limit.
type truncation struct {
	Truncated bool              `json:"truncated"`
	SizeKB    float64           `json:"sizeKB"`
	LimitKB   int               `json:"limitKB"`
	Hint      string            `json:"hint"`
	TopLevel  map[string]string `json:"topLevel"`
	Partial   any               `json:"partial,omitempty"`
}

// responseCap enforces the size limit in one place (a receiving middleware) on every tool answer, on every
// resource read (wow://<character>/<section> returns whole sections, the same data as wow_section) and on every
// prompt (prompts/get): a prompt keeps its instruction and rules, and only the embedded JSON data becomes the envelope.
type responseCap struct {
	limitKB int                 // 0 = no limit
	args    map[string][]string // tool name -> names of its arguments (from the input type)
}

func (c responseCap) middleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		res, err := next(ctx, method, req)
		if err != nil || c.limitKB <= 0 {
			return res, err
		}
		if read, ok := res.(*mcp.ReadResourceResult); ok && method == "resources/read" {
			c.applyResource(read)
			return res, err
		}
		if prompt, ok := res.(*mcp.GetPromptResult); ok && method == "prompts/get" {
			c.applyPrompt(prompt)
			return res, err
		}
		if method != "tools/call" {
			return res, err
		}
		call, ok := req.(*mcp.CallToolRequest)
		out, isResult := res.(*mcp.CallToolResult)
		if ok && isResult && !out.IsError {
			c.apply(call.Params.Name, out)
		}
		return res, err
	}
}

// apply replaces the JSON text of an answer that is too big. The WARNING line (another content item) is kept.
func (c responseCap) apply(tool string, res *mcp.CallToolResult) {
	for i := len(res.Content) - 1; i >= 0; i-- {
		text, ok := res.Content[i].(*mcp.TextContent)
		if !ok || strings.HasPrefix(text.Text, "WARNING:") {
			continue
		}
		if short, changed := truncateAnswer(text.Text, c.limitKB, narrowHint(tool, c.args[tool])); changed {
			text.Text = short
		}
		return
	}
}

// resourceHint is the narrowing hint for a resource over the limit: the same section can be read in pieces with wow_section.
const resourceHint = "The resource is over the response limit. Read the same section in pieces with the wow_section tool: section, character, " +
	"path (a dot path into the section) and limit (the first N items or keys at the end of the path). The server owner can raise the limit with --max-response-kb."

// applyResource replaces the JSON text of every resource content that is too big.
func (c responseCap) applyResource(res *mcp.ReadResourceResult) {
	for _, content := range res.Contents {
		if content == nil || content.Text == "" {
			continue
		}
		if short, changed := truncateAnswer(content.Text, c.limitKB, resourceHint); changed {
			content.Text = short
		}
	}
}

// promptHint is the narrowing hint for the data of a prompt over the limit.
const promptHint = "The data embedded in this prompt is over the response limit. Follow the instruction above with the numbers in partial, " +
	"and fetch the details you need with the wow_* tools (wow_section reads any part of the dump with path and limit). " +
	"The server owner can raise the limit with --max-response-kb."

// Prompt text is instruction + rules + a fenced JSON block (prompts.go, render). An answer over the limit keeps the text
// around the block and replaces only the JSON with the same envelope the tools get, so the prompt stays readable.
const (
	promptDataOpen  = "```json\n"
	promptDataClose = "\n```"
	minPromptData   = 1024 // the envelope never gets less than this, even when the text around the data is long
)

// applyPrompt caps the text of every message of a prompt result.
func (c responseCap) applyPrompt(res *mcp.GetPromptResult) {
	for _, msg := range res.Messages {
		if msg == nil {
			continue
		}
		if text, ok := msg.Content.(*mcp.TextContent); ok {
			text.Text = capPromptText(text.Text, c.limitKB)
		}
	}
}

// capPromptText returns text unchanged when it fits. Otherwise the JSON data block becomes a truncation envelope that fits
// the rest of the budget; a text with no JSON block (not produced by our prompts) is cut at a character boundary.
func capPromptText(text string, limitKB int) string {
	limit := limitKB * 1024
	if limitKB <= 0 || len(text) <= limit {
		return text
	}
	open := strings.Index(text, promptDataOpen)
	end := strings.LastIndex(text, promptDataClose)
	if open >= 0 && end > open {
		start := open + len(promptDataOpen)
		payload := text[start:end]
		budget := max(limit-(len(text)-len(payload)), minPromptData)
		if short, changed := truncateJSON(payload, budget, limitKB, promptHint); changed {
			return text[:start] + short + text[end:]
		}
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + "\n\n[The prompt was cut at the response limit. " + promptHint + "]"
}

// narrowHint tells the model how to ask for less, from the arguments the tool really has.
func narrowHint(tool string, args []string) string {
	const raise = " The server owner can raise the limit with --max-response-kb."
	if tool == "wow_section" {
		return "The answer is over the response limit. Read the section in pieces: path is a dot path into the section (a topLevel key without the leading section name, then deeper, " +
			"e.g. path=namespaces.C_AuctionHouse for the probe section; array items by 0-based index), and limit caps the items or keys listed at the end of the path (page by raising limit)." + raise
	}
	var narrowing []string
	for _, a := range args {
		if a != "character" {
			narrowing = append(narrowing, a)
		}
	}
	if len(narrowing) == 0 {
		return "The answer is over the response limit and " + tool + " has no narrowing arguments. Read the underlying data in pieces with wow_section (path, limit); " +
			"topLevel shows which parts are big." + raise
	}
	return "The answer is over the response limit. Ask for less with the arguments of " + tool + ": " + strings.Join(narrowing, ", ") +
		" (for example a query, a filter or a limit). topLevel shows which parts of the answer are big." + raise
}

// argNames lists the JSON argument names of a tool input type (embedded structs are flattened like encoding/json does).
func argNames[In any]() []string {
	var names []string
	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct {
			return
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			switch {
			case name == "-":
			case f.Anonymous && name == "":
				walk(f.Type)
			case !f.IsExported():
			case name == "":
				names = append(names, f.Name)
			default:
				names = append(names, name)
			}
		}
	}
	walk(reflect.TypeFor[In]())
	sort.Strings(names)
	return names
}

// truncateAnswer returns the envelope for a JSON answer over limitKB; changed = false when it fits (or is not JSON).
func truncateAnswer(text string, limitKB int, hint string) (string, bool) {
	if limitKB <= 0 {
		return text, false
	}
	return truncateJSON(text, limitKB*1024, limitKB, hint)
}

// truncateJSON is truncateAnswer with the budget in bytes; limitKB is only reported in the envelope.
func truncateJSON(text string, limit, limitKB int, hint string) (string, bool) {
	if len(text) <= limit {
		return text, false
	}
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var root any
	if err := dec.Decode(&root); err != nil {
		return text, false
	}
	env := truncation{
		Truncated: true,
		SizeKB:    math.Round(float64(len(text))/1024*10) / 10,
		LimitKB:   limitKB,
		Hint:      hint,
		TopLevel:  topLevel(root),
	}
	for _, n := range partialSteps {
		env.Partial = digest(root, n, 0)
		if out := marshalEnvelope(env); len(out) <= limit {
			return out, true
		}
	}
	env.Partial = nil
	return marshalEnvelope(env), true
}

func marshalEnvelope(env truncation) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(env); err != nil {
		return `{"truncated": true}`
	}
	return strings.TrimRight(buf.String(), "\n")
}

// topLevel summarizes the big parts of the answer: containers and long strings, with their size.
func topLevel(root any) map[string]string {
	obj, ok := root.(map[string]any)
	if !ok {
		return map[string]string{"(root)": describe(root)}
	}
	type entry struct {
		key  string
		size int
	}
	var entries []entry
	descriptions := map[string]string{}
	add := func(key string, v any) {
		entries = append(entries, entry{key, jsonSize(v)})
		descriptions[key] = describe(v)
	}
	for k, v := range obj {
		if !isContainer(v) && !isLongString(v) {
			continue
		}
		add(k, v)
		// A big object is opened one level down ("parent.child"), so a single wrapper key does not hide what is big.
		if child, ok := v.(map[string]any); ok && jsonSize(v) > bigObjectBytes {
			for ck, cv := range child {
				if isContainer(cv) || isLongString(cv) {
					add(k+"."+ck, cv)
				}
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].size != entries[j].size {
			return entries[i].size > entries[j].size
		}
		return entries[i].key < entries[j].key
	})
	out := map[string]string{}
	for i, e := range entries {
		if i == maxTopLevelEntries {
			out["..."] = fmt.Sprintf("%d more keys not listed", len(entries)-maxTopLevelEntries)
			break
		}
		out[e.key] = descriptions[e.key]
	}
	return out
}

func isContainer(v any) bool {
	switch v.(type) {
	case map[string]any, []any:
		return true
	}
	return false
}

func isLongString(v any) bool { s, ok := v.(string); return ok && len(s) > maxDigestString }

func jsonSize(v any) int {
	b, _ := json.Marshal(v)
	return len(b)
}

func sizeText(v any) string { return fmt.Sprintf("%.1f KB", float64(jsonSize(v))/1024) }

func describe(v any) string {
	switch x := v.(type) {
	case []any:
		return fmt.Sprintf("array, %d items, %s", len(x), sizeText(v))
	case map[string]any:
		return fmt.Sprintf("object, %d keys, %s", len(x), sizeText(v))
	case string:
		return fmt.Sprintf("string, %d characters", utf8.RuneCountInString(x))
	}
	return "value"
}

// digest keeps the shape of v but at most n items per array and 2n+4 keys per object, with the counts of what was cut.
func digest(v any, n, depth int) any {
	switch x := v.(type) {
	case map[string]any:
		if depth >= maxDigestDepth {
			return describe(v)
		}
		return digestObject(x, n, depth)
	case []any:
		if depth >= maxDigestDepth {
			return describe(v)
		}
		return digestItems(x, n, depth)
	case string:
		return cutString(x, n)
	}
	return v
}

func digestItems(list []any, n, depth int) []any {
	keep := min(len(list), n)
	out := make([]any, keep)
	for i := range out {
		out[i] = digest(list[i], n, depth+1)
	}
	return out
}

func digestObject(obj map[string]any, n, depth int) map[string]any {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	maxKeys := 2*n + 4
	cut := len(keys) > maxKeys
	if cut { // keep the smallest values: scalars and meta fields first
		sizes := map[string]int{}
		for _, k := range keys {
			sizes[k] = jsonSize(obj[k])
		}
		sort.SliceStable(keys, func(i, j int) bool { return sizes[keys[i]] < sizes[keys[j]] })
		keys = keys[:maxKeys]
	}
	out := map[string]any{}
	for _, k := range keys {
		switch list := obj[k].(type) {
		case []any:
			out[k] = digest(list, n, depth)
			if len(list) > n && depth < maxDigestDepth {
				addCount(out, k+"Total", len(list))
				addCount(out, k+"Shown", n)
			}
		default:
			out[k] = digest(obj[k], n, depth+1)
		}
	}
	if cut {
		addCount(out, "keysTotal", len(obj))
		addCount(out, "keysShown", len(keys))
	}
	return out
}

func addCount(out map[string]any, key string, n int) {
	if _, taken := out[key]; !taken {
		out[key] = n
	}
}

func cutString(s string, n int) string {
	limit := maxDigestString
	if n < 5 {
		limit = 120
	}
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "..."
}
