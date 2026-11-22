// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package storage

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"wowmcp/internal/snapshot"
)

// Default history settings.
const (
	// DefaultCompressDays: snapshots older than this many days are gzip-compressed.
	DefaultCompressDays = 7
	// compactBatch: the maximum number of files one Save call compresses (bounded work).
	compactBatch = 20

	plainExt = ".json"
	gzipExt  = ".json.gz"
)

// History stores previous character snapshots so that it can show what
// changed between them. It writes only to the server's own directory (by default
// "history" next to wow-mcp.exe); it never touches the game directory.
//
// Layout: <Dir>/<character>/<dumpedAt>.json (fresh) or <dumpedAt>.json.gz (older than
// CompressDays days), one file per snapshot, at most Keep files per character and at most
// MaxMB megabytes per character. Reading handles both formats the same way.
// It implements the progress.History port.
type History struct {
	Dir  string
	Keep int
	// MaxMB is the size limit of one character's directory in MB (0 = no limit).
	// When it is exceeded, the oldest snapshots go first; the newest always stays.
	MaxMB int
	// CompressDays: snapshots older than this many days are compressed (0 = DefaultCompressDays, <0 = disabled).
	CompressDays int
	// Now is the time source (nil = time.Now); injected in tests.
	Now func() time.Time

	// unloaded remembers which files are dumps from an empty client (snapshot.IsUnloadedDump),
	// so List does not load every file on every call. Key: path and size.
	mu       sync.Mutex
	unloaded map[string]bool

	// writeMu serializes Save (write, compression, pruning): MCP requests may arrive concurrently, and every
	// tool call records the dump it read.
	writeMu sync.Mutex
}

// HistoryOption changes History settings when created by NewHistory.
type HistoryOption func(*History)

// WithMaxMB sets the size limit of a character directory in MB (0 = no limit).
func WithMaxMB(mb int) HistoryOption { return func(h *History) { h.MaxMB = mb } }

// WithCompressDays sets the age (days) from which snapshots are compressed (<0 = never).
func WithCompressDays(days int) HistoryOption { return func(h *History) { h.CompressDays = days } }

// WithClock replaces the clock.
func WithClock(now func() time.Time) HistoryOption { return func(h *History) { h.Now = now } }

// NewHistory creates a history with sensible defaults (compression after 7 days, no MB limit).
func NewHistory(dir string, keep int, opts ...HistoryOption) *History {
	h := &History{Dir: dir, Keep: keep}
	for _, o := range opts {
		o(h)
	}
	return h
}

func (h *History) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

// entry is one remembered snapshot.
type entry struct {
	dumpedAt int64
	path     string
	size     int64
}

func (e entry) gzipped() bool { return strings.HasSuffix(e.path, gzipExt) }

// Fields not worth keeping in the history: the list of API functions says nothing about progress.
var historySkip = map[string]bool{"probe": true}

// characterDir returns the character directory inside Dir (characterDirName).
func (h *History) characterDir(character string) string {
	return filepath.Join(h.Dir, characterDirName(character))
}

// maxDirName caps a character directory name in bytes. Real keys ("Name-Realm") are far shorter; most file systems
// refuse names over 255 bytes.
const maxDirName = 100

// windowsDevices are names Windows reserves for devices in every directory (also with an extension, e.g. "CON.x").
var windowsDevices = map[string]bool{"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true}

// characterDirName turns a character key from the dump (text the server does not control) into one safe path
// element: no separators or characters Windows forbids, no "." or "..", no trailing dot or space (Windows drops them),
// no device name, and at most maxDirName bytes (a longer name is cut and gets a hash of the whole key, so two long
// keys do not share a directory). Ordinary keys such as "Piekasz-Realm" are returned unchanged.
func characterDirName(character string) string {
	clean := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 || r == 0x7f {
			return '_'
		}
		return r
	}, character)
	trimmed := strings.TrimRight(clean, ". ")
	clean = trimmed + strings.Repeat("_", len(clean)-len(trimmed))
	if clean == "" {
		clean = "_"
	}
	stem, _, _ := strings.Cut(clean, ".")
	if windowsDevices[strings.ToUpper(strings.TrimRight(stem, " "))] {
		clean = "_" + clean
	}
	if len(clean) > maxDirName {
		sum := fnv.New32a()
		sum.Write([]byte(character))
		cut := maxDirName - 9
		for cut > 0 && !utf8.RuneStart(clean[cut]) {
			cut--
		}
		clean = fmt.Sprintf("%s~%08x", clean[:cut], sum.Sum32())
	}
	return clean
}

// Record saves the snapshots of all characters in the database that are not yet in the history.
// Write errors do not interrupt the tools; they go to the log only.
func (h *History) Record(db *snapshot.Database) {
	if h == nil || h.Dir == "" {
		return
	}
	for _, name := range db.CharacterNames() {
		if err := h.Save(name, db.Characters[name]); err != nil {
			log.Printf("history: %s: %v", name, err)
		}
	}
}

// Save writes one character snapshot if it is not there yet, and trims the history to Keep.
func (h *History) Save(character string, s snapshot.Snapshot) error {
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	dumpedAt, ok := s.DumpedAt()
	if !ok || dumpedAt <= 0 {
		return nil // a snapshot without a time (old addon): there is no way to tell it from the next ones
	}
	dir := h.characterDir(character)
	name := strconv.FormatInt(dumpedAt, 10)
	path := filepath.Join(dir, name+plainExt)
	for _, p := range []string{path, filepath.Join(dir, name+gzipExt)} {
		if _, err := os.Stat(p); err == nil {
			return nil
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	kept := map[string]any{}
	for k, v := range s {
		if !historySkip[k] {
			kept[k] = v
		}
	}
	data, err := json.Marshal(kept)
	if err != nil {
		return err
	}
	if err := writeAtomic(path, data); err != nil {
		return err
	}
	h.compact(character) // best effort: errors go to the log only, the originals stay
	return h.prune(character)
}

// prune removes the oldest snapshots beyond the Keep limit, then beyond the MaxMB limit
// (the newest snapshot always stays).
func (h *History) prune(character string) error {
	if h.Keep <= 0 && h.MaxMB <= 0 {
		return nil
	}
	entries, err := h.entries(character)
	if err != nil {
		return err
	}
	remove := func(e entry) error {
		if err := os.Remove(e.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	for h.Keep > 0 && len(entries) > h.Keep {
		if err := remove(entries[0]); err != nil {
			return err
		}
		entries = entries[1:]
	}
	if h.MaxMB > 0 {
		limit := int64(h.MaxMB) * 1024 * 1024
		var total int64
		for _, e := range entries {
			total += e.size
		}
		for total > limit && len(entries) > 1 {
			if err := remove(entries[0]); err != nil {
				return err
			}
			total -= entries[0].size
			entries = entries[1:]
		}
	}
	return nil
}

// compact gzips the oldest snapshots older than CompressDays (at most compactBatch
// per call). Each file: write to .tmp, rename, and only then remove the original;
// on any error the original stays and work continues.
func (h *History) compact(character string) {
	if h.CompressDays < 0 {
		return
	}
	days := h.CompressDays
	if days == 0 {
		days = DefaultCompressDays
	}
	cutoff := h.now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
	entries, err := h.entries(character)
	if err != nil {
		log.Printf("history: compression %s: %v", character, err)
		return
	}
	done := 0
	for _, e := range entries {
		if done >= compactBatch || e.dumpedAt >= cutoff {
			break // entries are sorted, the later ones are newer
		}
		if e.gzipped() {
			continue
		}
		done++
		if err := compressFile(e.path); err != nil {
			log.Printf("history: compression %s: %v", e.path, err)
		}
	}
}

// compressFile turns <x>.json into <x>.json.gz atomically.
func compressFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	target := strings.TrimSuffix(path, plainExt) + gzipExt
	if err := writeAtomic(target, buf.Bytes()); err != nil {
		return err
	}
	return os.Remove(path)
}

// writeAtomic writes data to a uniquely named temporary file next to path and renames it into place, so neither a crash
// nor a second server process writing the same snapshot leaves half a file. The temporary name ends in .tmp, which
// entries ignores.
func writeAtomic(path string, data []byte) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// entries returns the remembered character snapshots from the oldest. It handles .json and .json.gz;
// when both exist (interrupted compression), the .json file counts, and prune will remove only that entry.
func (h *History) entries(character string) ([]entry, error) {
	if h == nil || h.Dir == "" {
		return nil, nil
	}
	files, err := os.ReadDir(h.characterDir(character))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	byTime := map[int64]entry{}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		var stem string
		switch {
		case strings.HasSuffix(name, gzipExt):
			stem = strings.TrimSuffix(name, gzipExt)
		case strings.HasSuffix(name, plainExt):
			stem = strings.TrimSuffix(name, plainExt)
		default:
			continue
		}
		n, err := strconv.ParseInt(stem, 10, 64)
		if err != nil {
			continue
		}
		info, err := f.Info()
		if err != nil {
			continue
		}
		e := entry{dumpedAt: n, path: filepath.Join(h.characterDir(character), name), size: info.Size()}
		if old, dup := byTime[n]; dup && !old.gzipped() {
			continue // the .json file stays
		}
		byTime[n] = e
	}
	entries := make([]entry, 0, len(byTime))
	for _, e := range byTime {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].dumpedAt < entries[j].dumpedAt })
	return entries, nil
}

// List returns the times of the remembered character snapshots from the oldest, without dumps from an empty
// client (game exit, when the character was already unloaded): those stay on disk but do not
// describe the character, so they must not spoil gold, progress or the timeline. SkippedUnloaded counts them.
func (h *History) List(character string) ([]int64, error) {
	entries, err := h.entries(character)
	if err != nil {
		return nil, err
	}
	var times []int64
	for _, e := range entries {
		if !h.isUnloaded(e) {
			times = append(times, e.dumpedAt)
		}
	}
	return times, nil
}

// SkippedUnloaded returns how many remembered character snapshots List skips as dumps from an empty client.
func (h *History) SkippedUnloaded(character string) int {
	entries, err := h.entries(character)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if h.isUnloaded(e) {
			n++
		}
	}
	return n
}

// isUnloaded checks (once per file) for a dump from an empty client. A file that cannot be
// read is not skipped here: Load will see its error.
func (h *History) isUnloaded(e entry) bool {
	key := e.path + "|" + strconv.FormatInt(e.size, 10)
	h.mu.Lock()
	verdict, known := h.unloaded[key]
	h.mu.Unlock()
	if known {
		return verdict
	}
	s, err := e.load()
	verdict = err == nil && s.IsUnloadedDump()
	h.mu.Lock()
	if h.unloaded == nil {
		h.unloaded = map[string]bool{}
	}
	h.unloaded[key] = verdict
	h.mu.Unlock()
	return verdict
}

// Load reads the remembered character snapshot from a given moment. Numbers come back as
// int64/float64, just like from the Lua parser.
func (h *History) Load(character string, dumpedAt int64) (snapshot.Snapshot, error) {
	entries, err := h.entries(character)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.dumpedAt == dumpedAt {
			return e.load()
		}
	}
	return nil, fmt.Errorf("no snapshot %d of character %s in the history", dumpedAt, character)
}

// maxSnapshotBytes caps one history snapshot after decompression, so a damaged or hostile .json.gz (a "decompression
// bomb") cannot exhaust memory. It is the same cap as for the dump the snapshot came from.
const maxSnapshotBytes = MaxDumpBytes

func (e entry) load() (snapshot.Snapshot, error) {
	data, err := readLimited(e.path, maxSnapshotBytes)
	if err != nil {
		return nil, err
	}
	if e.gzipped() {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.path, err)
		}
		data, err = io.ReadAll(io.LimitReader(zr, maxSnapshotBytes+1))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.path, err)
		}
		if len(data) > maxSnapshotBytes {
			return nil, fmt.Errorf("%s: decompresses to more than %d MB; the file is not read", e.path, maxSnapshotBytes>>20)
		}
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var raw map[string]any
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("%s: %w", e.path, err)
	}
	snap, ok := NormalizeJSON(raw).(map[string]any)
	if !ok { // "{}" after normalization is an empty list, "null" is nil
		return nil, fmt.Errorf("%s: the snapshot is not a non-empty object", e.path)
	}
	return snap, nil
}

// NormalizeJSON turns json.Number into int64 or float64 and an empty map into []any,
// so data from the history has the same types as a freshly parsed Lua file.
func NormalizeJSON(v any) any {
	switch x := v.(type) {
	case json.Number:
		if n, err := x.Int64(); err == nil {
			return n
		}
		f, _ := x.Float64()
		return f
	case map[string]any:
		if len(x) == 0 {
			return []any{}
		}
		for k, item := range x {
			x[k] = NormalizeJSON(item)
		}
		return x
	case []any:
		for i, item := range x {
			x[i] = NormalizeJSON(item)
		}
		return x
	}
	return v
}

// CheckWritable verifies that the history directory can be written: it creates the directory if needed
// and a temporary file that is removed at once. Used by the wow_doctor diagnostics; nil means writable.
func (h *History) CheckWritable() error {
	if h == nil || h.Dir == "" {
		return errors.New("history is disabled")
	}
	if err := os.MkdirAll(h.Dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(h.Dir, ".writecheck-*")
	if err != nil {
		return err
	}
	name := f.Name()
	f.Close()
	return os.Remove(name)
}
