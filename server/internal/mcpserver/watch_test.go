// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package mcpserver

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"wowmcp/internal/game/auctions"
	"wowmcp/internal/game/eventlog"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

type fakeSource struct {
	mu      sync.Mutex
	mod     time.Time
	statErr error
	loadErr error
	loads   int
}

func (f *fakeSource) ModTime() (time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.mod, f.statErr
}

func (f *fakeSource) Load() (*snapshot.Database, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loads++
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	return &snapshot.Database{}, nil
}

func (f *fakeSource) set(mod time.Time, loadErr error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mod, f.loadErr = mod, loadErr
}

func (f *fakeSource) loadCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loads
}

type fakeLedger struct {
	progress.Series
	eventlog.Store
	auctions.PriceLog
	mu      sync.Mutex
	records int
}

func (l *fakeLedger) Record(*snapshot.Database, progress.History) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records++
}

func (l *fakeLedger) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.records
}

func TestWatcherRecordsOnlyChangedFilesAndRetriesErrors(t *testing.T) {
	t0 := time.Unix(1000, 0)
	src := &fakeSource{mod: t0}
	led := &fakeLedger{}
	w := &watcher{cfg: Config{Snapshots: src, Ledger: led}, timer: src}

	w.check()
	w.check()
	if src.loadCount() != 1 || led.count() != 1 {
		t.Fatalf("unchanged file: loads %d, records %d", src.loadCount(), led.count())
	}

	src.set(t0.Add(time.Second), errors.New("being written"))
	w.check()
	w.check()
	if src.loadCount() != 3 || led.count() != 1 {
		t.Fatalf("failed load must be retried: loads %d, records %d", src.loadCount(), led.count())
	}

	src.set(t0.Add(time.Second), nil)
	w.check()
	w.check()
	if src.loadCount() != 4 || led.count() != 2 {
		t.Fatalf("loads %d, records %d", src.loadCount(), led.count())
	}
}

func TestWatcherSurvivesAMissingFile(t *testing.T) {
	src := &fakeSource{statErr: errors.New("missing")}
	led := &fakeLedger{}
	w := &watcher{cfg: Config{Snapshots: src, Ledger: led}, timer: src}
	w.check()
	src.mu.Lock()
	src.statErr, src.mod = nil, time.Unix(5, 0)
	src.mu.Unlock()
	w.check()
	if src.loadCount() != 1 || led.count() != 1 {
		t.Fatalf("loads %d, records %d", src.loadCount(), led.count())
	}
}

func TestWatchPollsUntilCancelled(t *testing.T) {
	src := &fakeSource{mod: time.Unix(1, 0)}
	led := &fakeLedger{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		Watch(ctx, Config{Snapshots: src, Ledger: led}, time.Millisecond)
		close(done)
	}()
	for i := 2; led.count() < 3 && i < 2000; i++ {
		src.set(time.Unix(int64(i), 0), nil)
		time.Sleep(2 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not stop")
	}
	if led.count() < 3 {
		t.Fatalf("records = %d", led.count())
	}
}

func TestWatchIsOffWithoutStorageOrModTime(t *testing.T) {
	Watch(context.Background(), Config{Snapshots: &fakeSource{}}, time.Millisecond)
	Watch(context.Background(), Config{Snapshots: noModTime{}, Ledger: &fakeLedger{}}, time.Millisecond)
}

type noModTime struct{}

func (noModTime) Load() (*snapshot.Database, error) { return nil, errors.New("x") }
