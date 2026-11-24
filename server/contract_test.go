// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package main

// Contract test between the domain code and the snapshots (roadmap 83). What it guarantees and what it does not:
//
//  1. Every tool with default arguments runs without an error on every test snapshot
//     (addon stubs: modern, legacy, forever, and the real dump from the game).
//  2. Every snapshot section that the domain code reads by a literal name (s.Map("x"), s.List("x"),
//     s["x"] on snapshot variables) appears at the top level in at least one test
//     snapshot, or is on an explicit exception list with a justification. A typo in a section name
//     or a section that the addon (according to the snapshots) does not write fails this test.
//
// What it does NOT guarantee: field names inside sections (e.g. "minBuyout"), value types, names read
// through variables named differently than those listed here, or dynamically created sections (a name from
// an argument, as in wow_section). It is a net against the most common mistake, not proof of compatibility with the addon.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
)

// snapshotVars are the names of variables that hold a whole snapshot (not sections) in the domain code.
var snapshotVars = map[string]bool{"s": true, "snap": true, "current": true, "before": true, "after": true, "first": true, "last": true}

// optionalSections are sections read by the server that no test snapshot has (e.g. only from a newer addon);
// each with a reason. Currently empty: all sections read have data in the stubs or in the real dump.
var optionalSections = map[string]string{}

// innerFields are fields inside sections that the scanner mistakes for sections, because functions in progress/diff.go name
// variables s, before and after also when they hold a single quest, spell or character.
var innerFields = map[string]bool{"level": true, "name": true, "objectives": true, "rank": true, "xp": true, "xpMax": true}

// sectionsRead collects the literal names of sections read in the internal/game packages.
func sectionsRead(t *testing.T) map[string][]string {
	t.Helper()
	found := map[string][]string{} // section -> files
	fset := token.NewFileSet()
	err := filepath.Walk(filepath.Join("internal", "game"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		note := func(lit ast.Expr) {
			if b, ok := lit.(*ast.BasicLit); ok && b.Kind == token.STRING {
				name, _ := strconv.Unquote(b.Value)
				found[name] = append(found[name], path)
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr: // s.Map("x"), s.List("x")
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok && (sel.Sel.Name == "Map" || sel.Sel.Name == "List") && len(x.Args) == 1 {
					if id, ok := sel.X.(*ast.Ident); ok && snapshotVars[id.Name] {
						note(x.Args[0])
					}
				}
			case *ast.IndexExpr: // s["x"]
				if id, ok := x.X.(*ast.Ident); ok && snapshotVars[id.Name] {
					note(x.Index)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// Fields that describe the snapshot itself (not character sections), also read by name.
func isMeta(name string) bool {
	switch name {
	case "dumpedAt", "dumpedAtText", "errors", "reason", "addonVersion", "timings", "disabledSections", "lastSkippedLogoutAt":
		return true
	}
	return false
}

func TestSectionsReadExistInFixtures(t *testing.T) {
	have := map[string]bool{}
	for _, s := range []map[string]any{fixtures.Modern(t), fixtures.Forever(t), fixtures.Dede(t), fixtures.Real(t)} {
		for k := range s {
			have[k] = true
		}
	}
	read := sectionsRead(t)
	if len(read) < 20 {
		t.Fatalf("the scanner found only %d sections: it has probably stopped working", len(read))
	}
	var missing []string
	for name, files := range read {
		if have[name] || isMeta(name) || innerFields[name] {
			continue
		}
		if _, ok := optionalSections[name]; ok {
			continue
		}
		sort.Strings(files)
		missing = append(missing, name+" (read in "+files[0]+")")
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("the domain code reads sections that are in no test snapshot (a typo or a new section without test data). "+
			"Add data to the stubs or add it to optionalSections with a justification:\n  %s", strings.Join(missing, "\n  "))
	}
	for name := range optionalSections {
		if have[name] {
			t.Errorf("section %s is already in the test snapshots: remove it from optionalSections", name)
		}
	}
}

// No tool with default arguments may return an error on any snapshot with data (with history).
func TestEveryToolRunsOnEveryFixture(t *testing.T) {
	pinGlobals(t)
	var scenarios []goldenScenario
	for _, sc := range goldenScenarios() {
		if sc.history && !sc.missing && sc.age == 30*time.Minute {
			scenarios = append(scenarios, sc)
		}
	}
	if len(scenarios) != 6 {
		t.Fatalf("expected 6 scenarios (3 stubs + deDE + real + long history), got %d", len(scenarios))
	}
	for _, sc := range scenarios {
		env := newGoldenEnv(t, sc)
		ran := 0
		for _, c := range goldenCases() {
			if c.prompt || c.variant != "default" || c.realOnly {
				continue
			}
			ran++
			out := env.run(t, c)
			if strings.Contains(out, "# isError: true") || strings.Contains(out, "PROTOCOL ERROR") {
				t.Errorf("%s / %s: %.300s", sc.dir, c.name, out)
			}
		}
		if ran < 30 {
			t.Errorf("%s: only %d tools were run", sc.dir, ran)
		}
	}
}
