// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/storage"
)

func bigAnswer() map[string]any {
	items := make([]any, 500)
	for i := range items {
		items[i] = map[string]any{"id": i, "name": fmt.Sprintf("Item number %d with a longish name", i)}
	}
	names := map[string]any{}
	for i := 0; i < 200; i++ {
		names[fmt.Sprintf("key%03d", i)] = strings.Repeat("x", 50)
	}
	return map[string]any{"snapshot": map[string]any{"ageMinutes": 3}, "items": items, "names": names, "gold": 12, "note": "small"}
}

func pretty(t *testing.T, v any) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSmallAnswerIsUntouched(t *testing.T) {
	text := pretty(t, map[string]any{"a": 1})
	if out, changed := truncateAnswer(text, 48, "h"); changed || out != text {
		t.Errorf("changed = %v", changed)
	}
	if _, changed := truncateAnswer(pretty(t, bigAnswer()), 0, "h"); changed {
		t.Error("limit 0 must mean no cap")
	}
}

func TestBigAnswerBecomesEnvelope(t *testing.T) {
	text := pretty(t, bigAnswer())
	if len(text) < 20*1024 {
		t.Fatalf("test data too small: %d", len(text))
	}
	out, changed := truncateAnswer(text, 8, "narrow it")
	if !changed || len(out) > 8*1024 {
		t.Fatalf("changed=%v size=%d", changed, len(out))
	}
	var env struct {
		Truncated bool              `json:"truncated"`
		SizeKB    float64           `json:"sizeKB"`
		LimitKB   int               `json:"limitKB"`
		Hint      string            `json:"hint"`
		TopLevel  map[string]string `json:"topLevel"`
		Partial   map[string]any    `json:"partial"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err)
	}
	if !env.Truncated || env.LimitKB != 8 || env.SizeKB < 20 || env.Hint != "narrow it" {
		t.Errorf("envelope: %+v", env)
	}
	if !strings.HasPrefix(env.TopLevel["items"], "array, 500 items") || !strings.HasPrefix(env.TopLevel["names"], "object, 200 keys") {
		t.Errorf("topLevel: %v", env.TopLevel)
	}
	if _, scalar := env.TopLevel["gold"]; scalar {
		t.Error("scalars do not belong in topLevel")
	}
	p := env.Partial
	if p["gold"] != float64(12) || p["note"] != "small" || p["snapshot"] == nil {
		t.Errorf("small keys must be kept: %v", p)
	}
	if shown, total := p["itemsShown"], p["itemsTotal"]; total != float64(500) || shown == float64(500) || len(p["items"].([]any)) != int(shown.(float64)) {
		t.Errorf("array counts: shown=%v total=%v", shown, total)
	}
	if again, _ := truncateAnswer(text, 8, "narrow it"); again != out {
		t.Error("the envelope is not deterministic")
	}
}

func TestEnvelopeFallsBackToDigestOnly(t *testing.T) {
	huge := map[string]any{"list": make([]any, 5000)}
	for i := range huge["list"].([]any) {
		huge["list"].([]any)[i] = map[string]any{"a": strings.Repeat("y", 200)}
	}
	out, changed := truncateAnswer(pretty(t, huge), 1, "h")
	if !changed || len(out) > 1024 || !strings.Contains(out, `"truncated": true`) || !strings.Contains(out, "array, 5000 items") {
		t.Errorf("changed=%v len=%d\n%s", changed, len(out), out)
	}
}

func TestNarrowHintUsesToolArguments(t *testing.T) {
	h := narrowHint("wow_bank", []string{"character", "minQuality", "query", "sellable"})
	if !strings.Contains(h, "minQuality, query, sellable") || strings.Contains(h, "character,") {
		t.Errorf("hint: %s", h)
	}
	if h := narrowHint("wow_section", argNames[sectionArgs]()); !strings.Contains(h, "path") || !strings.Contains(h, "limit") {
		t.Errorf("section hint: %s", h)
	}
	if h := narrowHint("wow_status", nil); !strings.Contains(h, "wow_section") {
		t.Errorf("no-argument hint: %s", h)
	}
}

func TestArgNamesFlattenEmbeddedStructs(t *testing.T) {
	got := strings.Join(argNames[timelineArgs](), ",")
	if !strings.Contains(got, "character") || len(argNames[noArgs]()) != 0 {
		t.Errorf("argNames = %s", got)
	}
}

func TestRealProbeIsCappedAndPageable(t *testing.T) {
	raw, err := os.ReadFile(fixtures.Testdata("real/probe_2026-11-15_addon070.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	text := pretty(t, v)
	if len(text) < 48*1024 {
		t.Fatalf("the probe answer is only %d bytes", len(text))
	}
	out, changed := truncateAnswer(text, 48, narrowHint("wow_section", argNames[sectionArgs]()))
	if !changed || len(out) > 48*1024 {
		t.Fatalf("changed=%v size=%d", changed, len(out))
	}
	var env struct {
		TopLevel map[string]string `json:"topLevel"`
		Partial  map[string]any    `json:"partial"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(env.TopLevel["probe"], "object") || env.Partial["probe"] == nil {
		t.Errorf("probe digest missing: %v", env.TopLevel)
	}
}

// Through the real protocol: a cap of 1 KB turns wow_status into an envelope, a limit of 0 does not.
func TestMiddlewareCapsToolAnswers(t *testing.T) {
	dump := fixtures.Testdata("WowMcp_forever.lua")
	call := func(limitKB int, tool string, args map[string]any) string {
		t.Helper()
		server := New(Config{Name: "wow-mcp", Version: "test", Snapshots: &storage.Store{File: dump}, Freshness: fixtures.At(0), MaxResponseKB: limitKB})
		ct, st := mcp.NewInMemoryTransports()
		ctx := context.Background()
		if _, err := server.Connect(ctx, st, nil); err != nil {
			t.Fatal(err)
		}
		cs, err := mcp.NewClient(&mcp.Implementation{Name: "t"}, nil).Connect(ctx, ct, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer cs.Close()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("%s: %v %v", tool, err, res)
		}
		return res.Content[len(res.Content)-1].(*mcp.TextContent).Text
	}
	capped := call(1, "wow_bags", nil)
	if len(capped) > 1024 || !strings.Contains(capped, `"truncated": true`) || !strings.Contains(capped, "wow_bags") {
		t.Errorf("not capped: %d bytes\n%.400s", len(capped), capped)
	}
	if uncapped := call(0, "wow_bags", nil); strings.Contains(uncapped, `"truncated"`) || len(uncapped) < 2048 {
		t.Errorf("limit 0 must not cap: %d bytes", len(uncapped))
	}
}

// Prompts: the instruction and rules stay, the embedded JSON becomes the envelope, and the whole text fits the limit.
func TestCapPromptTextKeepsInstructionAndShrinksData(t *testing.T) {
	items := make([]map[string]any, 400)
	for i := range items {
		items[i] = map[string]any{"name": fmt.Sprintf("Item %d", i), "note": strings.Repeat("x", 60)}
	}
	payload, _ := json.MarshalIndent(map[string]any{"items": items}, "", "  ")
	text := "TASK: plan.\n\nRULES.\n\nData:\n\n```json\n" + string(payload) + "\n```\n"
	if capPromptText(text, 0) != text {
		t.Fatal("limit 0 must not cap")
	}
	out := capPromptText(text, 8)
	if len(out) > 8*1024 || !strings.HasPrefix(out, "TASK: plan.\n\nRULES.") || !strings.HasSuffix(out, "\n```\n") {
		t.Fatalf("shape: %d bytes\n%.200s", len(out), out)
	}
	if !strings.Contains(out, `"truncated": true`) || !strings.Contains(out, "wow_section") {
		t.Errorf("no envelope:\n%.600s", out)
	}
	small := "TASK\n\n```json\n{\"a\": 1}\n```\n"
	if capPromptText(small, 8) != small {
		t.Error("a small prompt must stay untouched")
	}
	// No JSON block: cut on a character boundary with a note.
	plain := strings.Repeat("ą", 3000)
	cut := capPromptText(plain, 1)
	if !utf8.ValidString(cut) || !strings.Contains(cut, "cut at the response limit") || len(cut) > 1024+len(promptHint)+100 {
		t.Errorf("plain cut: %d bytes valid=%v", len(cut), utf8.ValidString(cut))
	}
}

// Through the real protocol: a prompt over the limit is capped, with the limit at 0 it is not.
func TestMiddlewareCapsPrompts(t *testing.T) {
	dump := fixtures.Testdata("WowMcp_forever.lua")
	get := func(limitKB int) string {
		t.Helper()
		server := New(Config{Name: "wow-mcp", Version: "test", Snapshots: &storage.Store{File: dump}, Freshness: fixtures.At(0), MaxResponseKB: limitKB})
		ct, st := mcp.NewInMemoryTransports()
		ctx := context.Background()
		if _, err := server.Connect(ctx, st, nil); err != nil {
			t.Fatal(err)
		}
		cs, err := mcp.NewClient(&mcp.Implementation{Name: "t"}, nil).Connect(ctx, ct, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer cs.Close()
		res, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: "gear_upgrades"})
		if err != nil {
			t.Fatal(err)
		}
		return res.Messages[0].Content.(*mcp.TextContent).Text
	}
	full := get(0)
	if strings.Contains(full, `"truncated"`) || len(full) < 3*1024 {
		t.Fatalf("limit 0 must not cap: %d bytes", len(full))
	}
	capped := get(2)
	if len(capped) > 2*1024 || !strings.Contains(capped, `"truncated": true`) || !strings.Contains(capped, "You only advise") {
		t.Errorf("not capped: %d bytes\n%.300s", len(capped), capped)
	}
}
