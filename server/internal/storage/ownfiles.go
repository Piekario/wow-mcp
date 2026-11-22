// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The server's own files (history, log) may be written only outside the game directory.
// This file guards that boundary and opens the log.

// ResolveBeside turns a relative path into an absolute one against the base directory
// (here: the directory of the executable); absolute paths are left alone.
func ResolveBeside(path, base string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(base, path)
}

// InsideDir says whether path lies inside directory dir or is it. The comparison uses
// absolute paths with symlinks resolved, and case is judged by
// filepath (case-insensitive on Windows). When it cannot be determined, it returns false.
func InsideDir(path, dir string) bool {
	p, err1 := canonical(path)
	d, err2 := canonical(dir)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(d, p)
	if err != nil { // e.g. different drives on Windows
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// canonical returns an absolute path with symlinks resolved in the existing
// part (the rest may not exist yet, like a log file before the first write).
func canonical(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	existing, rest := abs, ""
	for {
		if resolved, err := filepath.EvalSymlinks(existing); err == nil {
			return filepath.Join(resolved, rest), nil
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return abs, nil
		}
		rest = filepath.Join(filepath.Base(existing), rest)
		existing = parent
	}
}

// RequireOutside returns an error when path (a path the server will write to; what
// describes it in the message) lies inside any of the game directories. An empty path or game directory means no restriction.
func RequireOutside(what, path string, gameDirs ...string) error {
	if path == "" {
		return nil
	}
	for _, gameDir := range gameDirs {
		if gameDir != "" && InsideDir(path, gameDir) {
			return fmt.Errorf("%s %q is inside the game directory %q, and the server writes nothing there; point to a path outside it",
				what, path, gameDir)
		}
	}
	return nil
}

// GameDirOf returns the game directory that holds a dump file given with --file, so the write guard also works when
// the server is started without --wow-dir: for <game>/<_client_>/WTF/Account/<account>/SavedVariables/WowMcp.lua
// it is <game> (the folder above the client folder), for <dir>/WTF/... it is <dir>. "" when the path has no WTF folder.
func GameDirOf(file string) string {
	if file == "" {
		return ""
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		return ""
	}
	for dir := filepath.Dir(abs); ; dir = filepath.Dir(dir) {
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		if strings.EqualFold(filepath.Base(dir), "WTF") {
			if isClientFolder(filepath.Base(parent)) {
				return filepath.Dir(parent)
			}
			return parent
		}
	}
}

// OpenAppend opens a file for appending (creating it and any missing directories).
func OpenAppend(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
}

// Writable says whether the server can create files in dir (it creates and removes a probe file).
func Writable(dir string) bool {
	if dir == "" {
		return false
	}
	f, err := os.CreateTemp(dir, ".wow-mcp-write-probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}

// DefaultDir picks the server's own directory for the history, goals and log: the executable's directory when
// the server can write there (the usual portable setup), otherwise <user config directory>/wow-mcp (e.g. an
// exe installed in a read-only Program Files or /usr/local/bin). "" when neither is known.
func DefaultDir(exeDir string, userConfigDir func() (string, error)) string {
	if Writable(exeDir) {
		return exeDir
	}
	if userConfigDir != nil {
		if base, err := userConfigDir(); err == nil && base != "" {
			return filepath.Join(base, "wow-mcp")
		}
	}
	return exeDir
}
