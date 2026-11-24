// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShareDirSavesOnlyPlainFileNames(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "shared")
	path, err := ShareDir{Dir: dir}.Save("dump.json", []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); path != filepath.Join(dir, "dump.json") || string(data) != "{}" {
		t.Errorf("saved %q with %q", path, data)
	}
	for _, name := range []string{"", "../escape.json", filepath.Join("sub", "dump.json")} {
		if _, err := (ShareDir{Dir: dir}).Save(name, nil); err == nil {
			t.Errorf("%q was accepted as a file name", name)
		}
	}
}
