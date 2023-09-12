/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/majewsky/portunus/internal/core"
)

// Adapter translates between the Portunus database and the disk store.
type Adapter struct {
	//NOTE: No mutex here. All FS access is done by the goroutine that calls
	//Run(), so we don't have any concurrency to deal with.
	nexus     core.Nexus
	storePath string
	//This contains the known contents of the store file. We maintain this to
	//avoid useless roundtrip writes from disk -> nexus -> disk.
	diskState []byte
	//This is set when we signal ErrDatabaseNeedsInitialization to the nexus, to
	//instruct Run() to wait for the response before continuing.
	initPending bool
}

// NewAdapter initializes an Adapter instance.
func NewAdapter(nexus core.Nexus, storePath string) *Adapter {
	return &Adapter{nexus: nexus, storePath: storePath}
}

// Run listens for and propagates changes to the Portunus database and the disk
// store until `ctx` expires. An error is returned if any write into the LDAP
// database fails.
func (a *Adapter) Run(ctx context.Context) error {
	//first read initializes the internal database from the pre-existing store
	//file (or marks that initialization is required)
	errs := a.nexus.Update(a.updateNexusByLoadingFromDisk, nil)
	if !errs.IsEmpty() {
		return fmt.Errorf("while loading database from disk store: %s", errs.Join(", "))
	}

	//writes get sent to us from whatever goroutine the nexus update is running on
	writeChan := make(chan core.Database, 1)
	a.nexus.AddListener(ctx, func(db core.Database) {
		writeChan <- db
	})

	//if we instructed the nexus to perform first-time initialization, we need to
	//collect the respective update immediately; otherwise the file watcher setup
	//will fail on ENOENT
	if a.initPending {
		select {
		case <-ctx.Done():
			return nil
		case db := <-writeChan:
			err := a.writeDatabase(db)
			if err != nil {
				return err
			}
		}
	}

	//for further reads, we need a file watcher
	watcher, err := NewWatcher(a.storePath)
	if err != nil {
		return err
	}

LOOP:
	for {
		select {
		case <-ctx.Done():
			break LOOP
		case err := <-watcher.Backend.Errors:
			return fmt.Errorf("error while watching %s for changes: %w", a.storePath, err)
		case <-watcher.Backend.Events:
			//wait for whatever is updating the file to complete
			time.Sleep(25 * time.Millisecond)

			//load updated version of database from file
			errs := a.nexus.Update(a.updateNexusByLoadingFromDisk, nil)
			if !errs.IsEmpty() {
				return fmt.Errorf("while loading database from disk store: %s", errs.Join(", "))
			}

			//recreate the watcher (the original file might be gone if it was updated
			//by an atomic rename like we do in writeStoreFile())
			err = watcher.WhileSuspended(func() error { return nil })
			if err != nil {
				return err
			}
		case db := <-writeChan:
			//stop the watch while writing, to avoid picking up our own change
			err = watcher.WhileSuspended(func() error {
				return a.writeDatabase(db)
			})
			if err != nil {
				return err
			}
		}
	}

	return watcher.Close()
}

// persistedDatabase is a variant of type Database. This is what gets
// persisted into the database file.
type persistedDatabase struct {
	Users         []core.User  `json:"users"`
	Groups        []core.Group `json:"groups"`
	SchemaVersion uint         `json:"schema_version"`
}

func (a *Adapter) updateNexusByLoadingFromDisk(_ core.Database) (core.Database, error) {
	a.initPending = false
	buf, err := a.readStoreFile()
	if err != nil {
		if os.IsNotExist(err) {
			a.initPending = true
			return core.Database{}, core.ErrDatabaseNeedsInitialization
		}
		return core.Database{}, err
	}

	var pdb persistedDatabase
	err = json.Unmarshal(buf, &pdb)
	if err != nil {
		return core.Database{}, fmt.Errorf("cannot parse DB: %w", err)
	}

	if pdb.SchemaVersion != 1 {
		return core.Database{}, fmt.Errorf("found DB with schema version %d, but this Portunus only understands schema version 1", pdb.SchemaVersion)
	}

	return core.Database{
		Users:  pdb.Users,
		Groups: pdb.Groups,
	}, nil
}

func (a *Adapter) writeDatabase(db core.Database) error {
	pdb := persistedDatabase{
		Users:         db.Users,
		Groups:        db.Groups,
		SchemaVersion: 1,
	}
	buf, err := json.MarshalIndent(pdb, "", "  ")
	if err != nil {
		return err
	}
	buf = append(buf, '\n') //follow the Unix convention of having a NL at the end of the file
	return a.writeStoreFile(buf)
}

func (a *Adapter) readStoreFile() ([]byte, error) {
	buf, err := os.ReadFile(a.storePath)
	if err == nil {
		//remember the contents that were read to avoid a useless rewrite after
		//roundtrip into our own listener
		a.diskState = buf
	}
	return buf, err
}

func (a *Adapter) writeStoreFile(buf []byte) error {
	if bytes.Equal(buf, a.diskState) {
		//avoid pointless writes
		return nil
	}

	tmpPath := filepath.Join(
		filepath.Dir(a.storePath),
		fmt.Sprintf(".%s.%d", filepath.Base(a.storePath), os.Getpid()),
	)
	err := os.WriteFile(tmpPath, buf, 0666)
	if err != nil {
		return err
	}
	err = os.Rename(tmpPath, a.storePath)
	if err != nil {
		return err
	}

	a.diskState = buf
	return nil
}
