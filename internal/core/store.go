/*******************************************************************************
*
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
*
* This program is free software: you can redistribute it and/or modify it under
* the terms of the GNU General Public License as published by the Free Software
* Foundation, either version 3 of the License, or (at your option) any later
* version.
*
* This program is distributed in the hope that it will be useful, but WITHOUT ANY
* WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR
* A PARTICULAR PURPOSE. See the GNU General Public License for more details.
*
* You should have received a copy of the GNU General Public License along with
* this program. If not, see <http://www.gnu.org/licenses/>.
*
*******************************************************************************/

package core

import (
	"github.com/fsnotify/fsnotify"
	"github.com/majewsky/portunus/internal/core"
	"github.com/sapcc/go-bits/logg"
)

//Database contains the contents of Portunus' database. This is what gets
//persisted into the database file.
type Database struct {
	Users  []core.User
	Groups []core.Group
}

//FileStore is responsible for loading Portunus' database from
//PORTUNUS_SERVER_STATE_DIR, and persisting it when changes are made to it.
type FileStore struct {
	Path    string
	running bool
}

//FileStoreAPI is the interface that the engine uses to interact with the
//FileStore.
type FileStoreAPI struct {
	//Whenever the FileStore reads an updated version of the config file, it
	//sends the database contents into this channel.
	LoadEvents <-chan Database
	//Whenever the FileStore reads an updated version of the database from this
	//channel, it will persist that state into the database file.
	SaveRequests chan<- Database
}

//Run spawns the goroutine for the FileStore, and returns the API that the
//engine uses to interact with it.
func (s *FileStore) Run() *FileStoreAPI {
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
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logg.Fatal("cannot set up filesystem watcher for database: " + err.Error())
	}

	//TODO perform initial read of the database, initializing it empty if necessary

	for {
		select {
		case event := <-watcher.Events:
			//TODO
		case err := <-watcher.Errors:
			logg.Error(err.Error()) //TODO: should this be logg.Fatal()?
		case dbState := <-saveChan:
			//TODO
		}
	}
}
