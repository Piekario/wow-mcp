// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package savedvars

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// FuzzParse guards one thing: the parser neither panics nor hangs on any input
// (syntax errors are fine). To run it:
//
//	go test -run=^$ -fuzz=FuzzParse -fuzztime=75s ./internal/savedvars
func FuzzParse(f *testing.F) {
	fixtures, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "WowMcp*.lua"))
	for _, path := range fixtures {
		if src, err := os.ReadFile(path); err == nil {
			f.Add(src)
		}
	}
	for _, seed := range []string{
		"", "X", "X =", "X = {", "X = }", `X = "`, `X = "\`, "X = 0x", "X = -", "X = 1e", "X = .",
		"X = --[[", "--[[ ]] X = 1", "X = { [", "X = { [1", "X = { [1] = }", "X = { a = }",
		"X = {{{{}}}}", "X = -inf", "X = - nan", "X = 0xFFFFFFFFFFFFFFFFFF", "X = {[{}] = 1}",
		"\xEF\xBB\xBFX = 1", "X = 1;Y = 2",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		start := time.Now()
		_, _ = Parse(src)
		if d := time.Since(start); d > 2*time.Second {
			t.Fatalf("parsing %d bytes took %v", len(src), d)
		}
	})
}
