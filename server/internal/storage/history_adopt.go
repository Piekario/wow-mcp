// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package storage

import (
	"os"
	"path/filepath"
)

// Keys returns the names of the character directories in the history. A directory name is the character key unless
// the key had characters a file system refuses (characterDirName), which real "Name-Realm" keys do not.
func (h *History) Keys() ([]string, error) {
	if h == nil || h.Dir == "" {
		return nil, nil
	}
	dirs, err := os.ReadDir(h.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var keys []string
	for _, d := range dirs {
		if d.IsDir() {
			keys = append(keys, d.Name())
		}
	}
	return keys, nil
}

// Adopt copies the snapshots kept under the key from to the key to, skipping the times to already has. The from
// directory is left as it is, so a wrong match loses nothing.
func (h *History) Adopt(from, to string) error {
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	source, err := h.entries(from)
	if err != nil {
		return err
	}
	kept, err := h.entries(to)
	if err != nil {
		return err
	}
	have := map[int64]bool{}
	for _, e := range kept {
		have[e.dumpedAt] = true
	}
	dir := h.characterDir(to)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, e := range source {
		if have[e.dumpedAt] {
			continue
		}
		data, err := readLimited(e.path, maxSnapshotBytes)
		if err != nil {
			return err
		}
		if err := writeAtomic(filepath.Join(dir, filepath.Base(e.path)), data); err != nil {
			return err
		}
	}
	return nil
}
