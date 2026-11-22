// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package storage

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInsideDir(t *testing.T) {
	root := t.TempDir()
	game := filepath.Join(root, "WoW")
	if err := os.MkdirAll(filepath.Join(game, "WTF"), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, path string
		want       bool
	}{
		{"the same directory", game, true},
		{"file inside", filepath.Join(game, "WTF", "x.log"), true},
		{"nonexistent deeper", filepath.Join(game, "a", "b", "c.log"), true},
		{"escape via ..", filepath.Join(game, "WTF", "..", "..", "x.log"), false},
		{"sibling with a shared prefix", game + "-logs", false},
		{"above the directory", root, false},
		{"beside", filepath.Join(root, "history"), false},
	}
	for _, c := range cases {
		if got := InsideDir(c.path, game); got != c.want {
			t.Errorf("%s: InsideDir(%q) = %v, expected %v", c.name, c.path, got, c.want)
		}
	}
}

func TestInsideDirFollowsSymlinks(t *testing.T) {
	root := t.TempDir()
	game := filepath.Join(root, "WoW")
	if err := os.MkdirAll(game, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "shortcut")
	if err := os.Symlink(game, link); err != nil {
		t.Skip("no symlinks:", err)
	}
	if !InsideDir(filepath.Join(link, "x.log"), game) {
		t.Error("a file through a symlink to the game directory should count as inside the game directory")
	}
}

func TestInsideDirCaseFollowsPlatform(t *testing.T) {
	root := t.TempDir()
	game := filepath.Join(root, "World of Warcraft")
	if err := os.MkdirAll(game, 0o755); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(root, "WORLD OF WARCRAFT", "x.log")
	// Windows is case-insensitive, the rest are not: filepath decides.
	if got, want := InsideDir(other, game), runtime.GOOS == "windows"; got != want {
		t.Errorf("InsideDir with different letter case = %v, expected %v on %s", got, want, runtime.GOOS)
	}
}

func TestRequireOutside(t *testing.T) {
	game := t.TempDir()
	err := RequireOutside("log file", filepath.Join(game, "server.log"), game)
	if err == nil || !strings.Contains(err.Error(), "log file") || !strings.Contains(err.Error(), "game directory") {
		t.Errorf("no readable error: %v", err)
	}
	if err := RequireOutside("log file", filepath.Join(t.TempDir(), "server.log"), game); err != nil {
		t.Errorf("path outside the game: %v", err)
	}
	if err := RequireOutside("x", "anything", ""); err != nil {
		t.Errorf("without a game directory there are no restrictions: %v", err)
	}
	if err := RequireOutside("x", "", game); err != nil {
		t.Errorf("empty path (disabled): %v", err)
	}
}

func TestResolveBeside(t *testing.T) {
	base := filepath.Join(string(filepath.Separator), "opt", "wow")
	abs := filepath.Join(t.TempDir(), "x.log") // absolute on every OS
	if got := ResolveBeside("logs/x.log", base); got != filepath.Join(base, "logs", "x.log") {
		t.Errorf("relative: %q", got)
	}
	if got := ResolveBeside(abs, base); got != abs {
		t.Errorf("absolute: %q", got)
	}
	if got := ResolveBeside("", base); got != "" {
		t.Errorf("empty: %q", got)
	}
}

func TestOpenAppendCreatesAndAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "server.log")
	for _, line := range []string{"a\n", "b\n"} {
		f, err := OpenAppend(path)
		if err != nil {
			t.Fatal(err)
		}
		f.WriteString(line)
		f.Close()
	}
	if data, _ := os.ReadFile(path); string(data) != "a\nb\n" {
		t.Errorf("content = %q", data)
	}
}

func TestOwnFileReadMissingWriteReplace(t *testing.T) {
	f := OwnFile{Path: filepath.Join(t.TempDir(), "sub", "goals.json")}
	if data, err := f.Read(); data != nil || err != nil {
		t.Fatalf("missing file: %q %v (want nil, nil)", data, err)
	}
	for _, body := range []string{"first", "second, longer than the first"} {
		if err := f.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
		if got, err := f.Read(); err != nil || string(got) != body {
			t.Fatalf("read after write: %q %v", got, err)
		}
	}
	entries, _ := os.ReadDir(filepath.Dir(f.Path))
	if len(entries) != 1 {
		t.Errorf("temporary files remain after writing: %v", entries)
	}
	if f.Where() != f.Path {
		t.Errorf("Where = %q", f.Where())
	}
}

func TestOwnFileWriteFailsWhenDirIsAFile(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (OwnFile{Path: filepath.Join(blocker, "goals.json")}).Write([]byte("x")); err == nil {
		t.Error("writing into a directory that is a file should return an error")
	}
}

func TestDefaultDir(t *testing.T) {
	exe := t.TempDir()
	config := func() (string, error) { return "/home/u/.config", nil }
	if got := DefaultDir(exe, config); got != exe {
		t.Errorf("writable exe directory: %q", got)
	}
	readOnly := filepath.Join(t.TempDir(), "missing") // a directory that does not exist cannot be written to
	if got := DefaultDir(readOnly, config); got != filepath.Join("/home/u/.config", "wow-mcp") {
		t.Errorf("not writable: %q", got)
	}
	if got := DefaultDir(readOnly, func() (string, error) { return "", errors.New("no home") }); got != readOnly {
		t.Errorf("no config directory: %q", got)
	}
	if Writable("") || !Writable(exe) {
		t.Error("Writable")
	}
	if entries, _ := os.ReadDir(exe); len(entries) != 0 {
		t.Errorf("the probe file must be removed: %v", entries)
	}
}
