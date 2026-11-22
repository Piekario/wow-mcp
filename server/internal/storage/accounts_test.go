// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
)

// twoAccounts builds <wow>/_forever_/WTF/Account/{ALPHA,BETA}/SavedVariables with a dump each; BETA is the newer one.
func twoAccounts(t *testing.T) string {
	t.Helper()
	wow := t.TempDir()
	src, err := os.ReadFile(fixtures.Testdata("WowMcp.lua"))
	if err != nil {
		t.Fatal(err)
	}
	for i, acc := range []string{"ALPHA", "BETA"} {
		dir := filepath.Join(wow, "_forever_", "WTF", "Account", acc, "SavedVariables")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, SavedVariablesFile)
		if err := os.WriteFile(path, src, 0o644); err != nil {
			t.Fatal(err)
		}
		mod := time.Now().Add(time.Duration(i-5) * time.Hour)
		if err := os.Chtimes(path, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	return wow
}

func TestSeveralAccountsNewestWinsAndOthersAreReported(t *testing.T) {
	db, err := (&Store{WowDir: twoAccounts(t)}).Load()
	if err != nil {
		t.Fatal(err)
	}
	if db.Account != "BETA" || db.AccountPinned || len(db.OtherAccounts) != 1 || db.OtherAccounts[0] != "ALPHA" {
		t.Errorf("account=%q pinned=%v others=%v", db.Account, db.AccountPinned, db.OtherAccounts)
	}
}

func TestAccountPinWinsOverNewest(t *testing.T) {
	wow := twoAccounts(t)
	db, err := (&Store{WowDir: wow, Account: "alpha"}).Load() // case-insensitive, like Windows folders
	if err != nil {
		t.Fatal(err)
	}
	if db.Account != "ALPHA" || !db.AccountPinned || len(db.OtherAccounts) != 1 || db.OtherAccounts[0] != "BETA" {
		t.Errorf("account=%q pinned=%v others=%v", db.Account, db.AccountPinned, db.OtherAccounts)
	}
}

func TestUnknownPinnedAccountListsTheAvailableOnes(t *testing.T) {
	_, err := (&Store{WowDir: twoAccounts(t), Account: "GAMMA"}).Load()
	if err == nil || !strings.Contains(err.Error(), "GAMMA") || !strings.Contains(err.Error(), "ALPHA, BETA") {
		t.Errorf("error: %v", err)
	}
}

func TestSingleAccountAndExplicitFile(t *testing.T) {
	db, err := (&Store{WowDir: fakeWowDir(t, true)}).Load()
	if err != nil {
		t.Fatal(err)
	}
	if db.Account != "TESTACCOUNT" || len(db.OtherAccounts) != 0 {
		t.Errorf("account=%q others=%v", db.Account, db.OtherAccounts)
	}
	db, err = (&Store{File: fixtures.Testdata("WowMcp.lua")}).Load()
	if err != nil {
		t.Fatal(err)
	}
	if db.Account != "" {
		t.Errorf("a file outside WTF/Account has no account, got %q", db.Account)
	}
}

func TestHistoryCheckWritable(t *testing.T) {
	h := NewHistory(filepath.Join(t.TempDir(), "history"), 5)
	if err := h.CheckWritable(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(h.Dir)
	if len(entries) != 0 {
		t.Errorf("the probe file must be removed: %v", entries)
	}
	file := filepath.Join(t.TempDir(), "file")
	os.WriteFile(file, nil, 0o644)
	if err := NewHistory(file, 5).CheckWritable(); err == nil {
		t.Error("a path that is a file cannot be a history directory")
	}
	if err := (*History)(nil).CheckWritable(); err == nil {
		t.Error("nil history is disabled")
	}
}
