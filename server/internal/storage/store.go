// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package storage is the infrastructure: finding and reading the SavedVariables file
// from the game directory (read-only) and the snapshot history in the server's own directory.
package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"wowmcp/internal/savedvars"
	"wowmcp/internal/snapshot"
)

// SavedVariablesFile is the name of the file the game client saves the WowMcp addon variables to.
const SavedVariablesFile = "WowMcp.lua"

// Store knows where the SavedVariables file is and reloads it on every
// tool call. The file is a few dozen KB, so a cache is not
// needed, and this way /reload in game is immediately visible in the response.
type Store struct {
	WowDir string // the game directory or one client folder inside it (e.g. _forever_, _retail_, _classic_era_)
	Client string // optional: the client folder to read (e.g. "_classic_era_" or "classic_era"); empty = every client folder
	File   string // optional: the full path to the file, skips searching
	// Account optionally pins one WTF account folder name (e.g. "MYACCOUNT"; --account / WOW_ACCOUNT, case-insensitive).
	// Empty = the account whose dump was modified most recently.
	Account string

	// Sleep is the pause between read retries; nil means time.Sleep. Tests substitute a stub.
	Sleep func(time.Duration)
}

// retryDelays are the pauses before successive read attempts: 4 attempts in about 600 ms in total.
// The game rewrites SavedVariables on /reload and logout; a read at that moment may hit
// a sharing violation, an empty file or a truncated one.
var retryDelays = []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 300 * time.Millisecond}

func (s *Store) sleep(d time.Duration) {
	if s.Sleep != nil {
		s.Sleep(d)
		return
	}
	time.Sleep(d)
}

// found is the result of locating the dump: the file chosen and the account folders around it.
type found struct {
	path    string
	account string   // account folder of the chosen file ("" when the path has no WTF/Account/<name>/SavedVariables shape)
	others  []string // other account folders that have a dump, sorted
	pinned  bool     // the account was pinned with Store.Account
}

// accountOf returns the account folder name of <...>/WTF/Account/<ACCOUNT>/SavedVariables/<file>, or "".
func accountOf(path string) string {
	dir := filepath.Dir(path)
	if filepath.Base(dir) != "SavedVariables" {
		return ""
	}
	acc := filepath.Dir(dir)
	if filepath.Base(filepath.Dir(acc)) != "Account" {
		return ""
	}
	return filepath.Base(acc)
}

// locate returns the newest WowMcp.lua file and says which account it belongs to.
//
// Game directory layout: <WoW>/<client>/WTF/Account/<ACCOUNT>/SavedVariables/, where <client> is a folder like
// _retail_, _classic_, _classic_era_, _ptr_ or _forever_. We accept the main game directory (every client folder
// in it, or only Client when it is set) and a client folder itself.
// With several accounts the one pinned by Account wins; without a pin the most recently modified dump does,
// and the other account folders are reported so the choice is never silent.
func (s *Store) locate() (found, error) {
	if s.File != "" {
		if _, err := os.Stat(s.File); err != nil {
			return found{}, fmt.Errorf("cannot open file %q: %w", s.File, err)
		}
		return found{path: s.File, account: accountOf(s.File)}, nil
	}
	if s.WowDir == "" {
		return found{}, errors.New("no game directory given: start the server with --wow-dir or set the WOW_DIR variable")
	}
	if _, err := os.Stat(s.WowDir); err != nil {
		return found{}, fmt.Errorf("game directory %q does not exist or is not accessible: %w", s.WowDir, err)
	}

	roots, err := s.clientRoots()
	if err != nil {
		return found{}, err
	}
	var matches []string
	for _, root := range roots {
		list, err := filepath.Glob(filepath.Join(root, "WTF", "Account", "*", "SavedVariables", SavedVariablesFile))
		if err != nil {
			return found{}, err
		}
		matches = append(matches, list...)
	}
	if len(matches) == 0 {
		where := fmt.Sprintf("%q or its client folders (_retail_, _classic_era_, _forever_...)", s.WowDir)
		if s.Client != "" {
			where = fmt.Sprintf("the client folder %q of %q", ClientFolder(s.Client), s.WowDir)
		}
		return found{}, fmt.Errorf("there is no WTF/Account/<account>/SavedVariables/%s file in %s. "+
			"Check that the WowMcp addon is enabled and type /mcp reload in game", SavedVariablesFile, where)
	}
	return s.pick(matches)
}

// pick chooses one dump among the candidates: the pinned account, otherwise the most recently modified file.
func (s *Store) pick(matches []string) (found, error) {
	accounts := map[string]bool{}
	for _, m := range matches {
		accounts[accountOf(m)] = true
	}
	candidates := matches
	pin := strings.TrimSpace(s.Account)
	if pin != "" {
		candidates = nil
		for _, m := range matches {
			if strings.EqualFold(accountOf(m), pin) {
				candidates = append(candidates, m)
			}
		}
		if len(candidates) == 0 {
			return found{}, fmt.Errorf("no dump for the account %q (--account / WOW_ACCOUNT). Accounts with a %s file: %s",
				pin, SavedVariablesFile, strings.Join(sortedKeys(accounts), ", "))
		}
	}
	// Several clients of one account or several accounts: the most recently modified file wins.
	sort.SliceStable(candidates, func(i, j int) bool { return modTime(candidates[i]).After(modTime(candidates[j])) })
	chosen := candidates[0]
	res := found{path: chosen, account: accountOf(chosen), pinned: pin != ""}
	delete(accounts, res.account)
	res.others = sortedKeys(accounts)
	return res, nil
}

func sortedKeys(set map[string]bool) []string {
	var out []string
	for k := range set {
		if k != "" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// ClientFolder turns a client name into its folder name: "classic_era" and "_classic_era_" both give "_classic_era_".
func ClientFolder(client string) string {
	c := strings.TrimSpace(client)
	if c == "" {
		return ""
	}
	return "_" + strings.Trim(c, "_") + "_"
}

// isClientFolder says whether a directory name looks like a WoW client folder (_retail_, _classic_era_, _forever_...).
func isClientFolder(name string) bool {
	return len(name) > 2 && strings.HasPrefix(name, "_") && strings.HasSuffix(name, "_")
}

// clientRoots lists the directories that may hold WTF: the game directory itself and its client folders,
// or only the pinned client folder.
func (s *Store) clientRoots() ([]string, error) {
	if s.Client != "" {
		dir := filepath.Join(s.WowDir, ClientFolder(s.Client))
		if _, err := os.Stat(dir); err != nil {
			return nil, fmt.Errorf("client folder %q not found in the game directory %q (--client / WOW_CLIENT): %w", ClientFolder(s.Client), s.WowDir, err)
		}
		return []string{dir}, nil
	}
	roots := []string{s.WowDir}
	entries, err := os.ReadDir(s.WowDir)
	if err != nil {
		return nil, fmt.Errorf("cannot list the game directory %q: %w", s.WowDir, err)
	}
	for _, e := range entries {
		if e.IsDir() && isClientFolder(e.Name()) {
			roots = append(roots, filepath.Join(s.WowDir, e.Name()))
		}
	}
	return roots, nil
}

func modTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// MaxDumpBytes caps the SavedVariables file the server will parse. Real dumps are well under 1 MB; the cap keeps a
// damaged or hostile file from exhausting memory (the parser's own memory use is proportional to its input).
const MaxDumpBytes = 64 << 20

// readLimited reads a whole file, refusing one larger than limit bytes.
func readLimited(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s is larger than %d MB, which no real dump is; the file is not read", path, limit>>20)
	}
	return data, nil
}

// readRoot reads and parses the file once; it returns the WowMcpDB root.
func readRoot(path string) (map[string]any, error) {
	src, err := readLimited(path, MaxDumpBytes)
	if err != nil {
		return nil, err
	}
	vars, err := savedvars.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	root, ok := vars["WowMcpDB"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s contains no data. Type /mcp reload in game", path)
	}
	return root, nil
}

// readRootWithRetry retries the read when the file may be being written by the game right now.
func (s *Store) readRootWithRetry(path string) (map[string]any, error) {
	root, err := readRoot(path)
	for _, delay := range retryDelays {
		if err == nil {
			return root, nil
		}
		s.sleep(delay)
		root, err = readRoot(path)
	}
	if err != nil {
		return nil, fmt.Errorf("%w (after %d attempts; the game may be writing the file right now, try again in a moment)",
			err, len(retryDelays)+1)
	}
	return root, nil
}

// Load reads and parses the SavedVariables file and records every snapshot in the history.
// Configuration errors (missing directory or file) are returned immediately; read and syntax errors after retries.
func (s *Store) Load() (*snapshot.Database, error) {
	loc, err := s.locate()
	if err != nil {
		return nil, err
	}
	path := loc.path
	root, err := s.readRootWithRetry(path)
	if err != nil {
		return nil, err
	}

	db := &snapshot.Database{Path: path, ModTime: modTime(path), Characters: map[string]snapshot.Snapshot{},
		Account: loc.account, OtherAccounts: loc.others, AccountPinned: loc.pinned}
	db.Last, _ = root["last"].(string)
	if characters, ok := root["characters"].(map[string]any); ok {
		for name, raw := range characters {
			if s, ok := raw.(map[string]any); ok {
				db.Characters[name] = s
			}
		}
	}
	if len(db.Characters) == 0 {
		return nil, fmt.Errorf("%s contains no characters. Type /mcp reload in game", path)
	}
	return db, nil
}

// ModTime is the modification time of the dump file Load would read, without reading it.
func (s *Store) ModTime() (time.Time, error) {
	loc, err := s.locate()
	if err != nil {
		return time.Time{}, err
	}
	info, err := os.Stat(loc.path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
