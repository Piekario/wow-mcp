// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package mcpserver

import (
	"context"
	"log"
	"time"
)

// ModTimer is implemented by a snapshot source that can tell when its file changed without reading it.
type ModTimer interface {
	ModTime() (time.Time, error)
}

// Watch records every save of the dump file without waiting for a tool call: the game may save twice between two
// calls, and the first dump would never be seen. It polls the modification time every `every`, loads and records the
// file only when it changed, and returns when ctx is cancelled. Errors (file missing, game writing at that moment)
// are retried on the next tick and logged only when they change.
func Watch(ctx context.Context, cfg Config, every time.Duration) {
	timer, ok := cfg.Snapshots.(ModTimer)
	if !ok || (cfg.History == nil && cfg.Ledger == nil) {
		return
	}
	w := &watcher{cfg: cfg, timer: timer}
	w.check()
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			w.check()
		}
	}
}

type watcher struct {
	cfg     Config
	timer   ModTimer
	seen    time.Time
	lastErr string
}

func (w *watcher) check() {
	mod, err := w.timer.ModTime()
	if err == nil && mod.Equal(w.seen) {
		return
	}
	if err == nil {
		err = w.record()
	}
	w.report(err)
	if err == nil {
		w.seen = mod
	}
}

func (w *watcher) record() error {
	db, err := w.cfg.Snapshots.Load()
	if err != nil {
		return err
	}
	if w.cfg.History != nil {
		adoptFormerKeys(db, w.cfg.History, w.cfg.Ledger)
		w.cfg.History.Record(db)
	}
	if w.cfg.Ledger != nil {
		w.cfg.Ledger.Record(db, w.cfg.History)
	}
	return nil
}

func (w *watcher) report(err error) {
	text := ""
	if err != nil {
		text = err.Error()
	}
	if text != w.lastErr && text != "" {
		log.Print("save watcher: ", text)
	}
	w.lastErr = text
}
