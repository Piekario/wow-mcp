// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package storage

import (
	"fmt"
	"path/filepath"
)

// ShareDir is the server's own directory for anonymized dumps the player may publish (wow_share_dump). A path
// inside the game directory is rejected by main through RequireOutside before this type is created.
type ShareDir struct {
	Dir string
}

// Save writes one file into the directory and returns its full path. The name must be a bare file name.
func (d ShareDir) Save(name string, data []byte) (string, error) {
	if name == "" || filepath.Base(name) != name {
		return "", fmt.Errorf("%q is not a plain file name", name)
	}
	path := filepath.Join(d.Dir, name)
	if err := (OwnFile{Path: path}).Write(data); err != nil {
		return "", err
	}
	return path, nil
}
