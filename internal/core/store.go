/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sapcc/go-bits/logg"
)

// Database contains the contents of Portunus' database.
type Database struct {
	Users  []User
	Groups []Group
}

// persistedDatabase is a variant of type Database. This is what gets
// persisted into the database file.
type persistedDatabase struct {
	Users         []User  `json:"users"`
	Groups        []Group `json:"groups"`
	SchemaVersion uint    `json:"schema_version"`
}

// FileStore is responsible for loading Portunus' database from
// PORTUNUS_SERVER_STATE_DIR, and persisting it when changes are made to it.
//
// The Initializer function is called at most once, only when there is no
// existing database file at the given Path.
type FileStore struct {
	Path        string
	Initializer func() Database
	running     bool
}

// FileStoreAPI is the interface that the engine uses to interact with the
// FileStore.
type FileStoreAPI struct {
	//Whenever the FileStore reads an updated version of the config file, it
	//sends the database contents into this channel.
	LoadEvents <-chan Database
	//Whenever the FileStore reads an updated version of the database from this
	//channel, it will persist that state into the database file.
	SaveRequests chan<- Database
}

// RunAsync spawns the goroutine for the FileStore, and returns the API that the
// engine uses to interact with it.
func (s *FileStore) RunAsync() *FileStoreAPI {
	if s.running {
		panic("cannot call FileStore.Run() twice")
	}
	s.running = true

	loadChan := make(chan Database, 1)
	saveChan := make(chan Database, 1)
	go s.run(loadChan, saveChan)
	return &FileStoreAPI{LoadEvents: loadChan, SaveRequests: saveChan}
}

func (s *FileStore) run(loadChan chan<- Database, saveChan <-chan Database) {
	//perform initial read of the database
	loadChan <- s.loadDB(true)
	watcher := s.makeWatcher()

	for {
		select {
		case <-watcher.Events:
			//wait for whatever is updating the file to complete
			time.Sleep(25 * time.Millisecond)
			//load updated version of database from file
			loadChan <- s.loadDB(false)
			//recreate the watcher (the original file might be gone if it was updated
			//by an atomic rename() like we do in saveDB())
			s.cleanupWatcher(watcher)
			watcher = s.makeWatcher()
		case err := <-watcher.Errors:
			logg.Error(err.Error())
		case db := <-saveChan:
			//stop watching while we modify the database file, so as not to pick up
			//our own change
			s.cleanupWatcher(watcher)
			s.saveDB(db)
			watcher = s.makeWatcher()
		}
	}
}

func (s *FileStore) cleanupWatcher(watcher *fsnotify.Watcher) {
	err := watcher.Close()
	if err != nil {
		logg.Fatal("cannot cleanup filesystem watcher: " + err.Error())
	}
}

func (s *FileStore) makeWatcher() *fsnotify.Watcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logg.Fatal("cannot initialize filesystem watcher: " + err.Error())
	}
	err = watcher.Add(s.Path)
	if err != nil {
		logg.Fatal("cannot setup filesystem watch on %s: %s", s.Path, err.Error())
	}
	return watcher
}

func (s *FileStore) loadDB(allowEmpty bool) Database {
	dbContents, err := os.ReadFile(s.Path)
	if err != nil {
		//initialize empty DB on first run
		if os.IsNotExist(err) && allowEmpty {
			s.saveDB(s.Initializer())
			return s.loadDB(false)
		}
		logg.Fatal(err.Error())
	}

	var pdb persistedDatabase
	err = json.Unmarshal(dbContents, &pdb)
	if err != nil {
		logg.Fatal("cannot load main database: " + err.Error())
	}

	if pdb.SchemaVersion != 1 {
		logg.Fatal("found DB with schema version %d, but this Portunus only understands schema version 1", pdb.SchemaVersion)
	}

	//TODO validate DB (e.g. groups should only contain users that actually exist)
	return Database{
		Users:  pdb.Users,
		Groups: pdb.Groups,
	}
}

func (s *FileStore) saveDB(db Database) {
	tmpPath := filepath.Join(
		filepath.Dir(s.Path),
		fmt.Sprintf(".%s.%d", filepath.Base(s.Path), os.Getpid()),
	)

	//serialize with predictable order to minimize diffs
	sort.Slice(db.Groups, func(i, j int) bool {
		return db.Groups[i].Name < db.Groups[j].Name
	})
	sort.Slice(db.Users, func(i, j int) bool {
		return db.Users[i].LoginName < db.Users[j].LoginName
	})

	dbContents, err := json.Marshal(persistedDatabase{
		Users:         db.Users,
		Groups:        db.Groups,
		SchemaVersion: 1,
	})
	if err == nil {
		var buf bytes.Buffer
		err = json.Indent(&buf, dbContents, "", "\t")
		dbContents = buf.Bytes()
	}
	if err == nil {
		err = os.WriteFile(tmpPath, dbContents, 0644)
	}
	if err == nil {
		err = os.Rename(tmpPath, s.Path)
	}
	if err != nil {
		logg.Fatal(err.Error())
	}
}
