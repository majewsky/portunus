/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package store

import (
	"fmt"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches the database store file for changes. This wraps an
// fsnotify.Watcher with convenience functions for pausing the watch
// temporarily.
type Watcher struct {
	Backend   *fsnotify.Watcher
	storePath string
}

// NewWatcher initializes a new Watcher.
func NewWatcher(storePath string) (*Watcher, error) {
	backend, err := makeWatcherBackend(storePath)
	return &Watcher{backend, storePath}, err
}

// WhileSuspended disables the watcher, executes the action, then reenables the
// watcher unless the action failed.
func (w *Watcher) WhileSuspended(action func() error) error {
	err := w.Close()
	if err != nil {
		return err
	}

	err = action()
	if err != nil {
		return err
	}

	w.Backend, err = makeWatcherBackend(w.storePath)
	return err
}

// Close cleans up the watcher backend.
func (w *Watcher) Close() error {
	err := w.Backend.Close()
	if err != nil {
		return fmt.Errorf("cannot cleanup filesystem watcher: %w", err)
	}
	w.Backend = nil
	return nil
}

func makeWatcherBackend(storePath string) (*fsnotify.Watcher, error) {
	backend, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("cannot initialize filesystem watcher: %w", err)
	}
	err = backend.Add(storePath)
	if err != nil {
		return nil, fmt.Errorf("cannot setup filesystem watcher on %s: %w", storePath, err)
	}
	return backend, nil
}
