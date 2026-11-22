// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package storage

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// OwnFile is one of the server's own files (goals, stat weights) located next to the history directory.
// Writing is atomic: data goes to a temporary file in the same directory, then rename,
// so an interrupted write does not leave half a file. A path inside the game directory is rejected by main
// through RequireOutside before this type is created.
// It implements the goals.Store and items.WeightsSource ports.
type OwnFile struct {
	Path string
}

// Read returns the file contents; (nil, nil) when the file does not exist yet.
func (f OwnFile) Read() ([]byte, error) {
	data, err := os.ReadFile(f.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// Write replaces the whole file, creating missing directories.
func (f OwnFile) Write(data []byte) (err error) {
	dir := filepath.Dir(f.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(f.Path)+".*.tmp")
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
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), f.Path)
}

// Where returns the file path.
func (f OwnFile) Where() string { return f.Path }
