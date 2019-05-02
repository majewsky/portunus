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
	"sync"

	goldap "gopkg.in/ldap.v3"
)

//Entity is the interface satisfied by all our model classes (at the moment,
//Group and User).
type Entity interface {
	//IsEqualTo is similar to `this == other`, but does not consider computed
	//fields.
	IsEqualTo(other Entity) bool
	//Render this entity into an AddRequest for LDAP. The argument is the
	//PORTUNUS_LDAP_SUFFIX.
	RenderToLDAP(suffix string) goldap.AddRequest
}

//Event describes a change to the data model.
type Event struct {
	Added    []Entity
	Modified []Modification
	Deleted  []Entity
	//TODO .Modified, .Deleted
}

//Modification appears in type Event.
type Modification struct {
	Old Entity
	New Entity
}

//Engine is the core engine of portunus-server.
type Engine interface {
	FindUser(loginName string) *UserWithPerms
	ListGroups() []Group
	ListUsers() []User
}

//engine implements the Engine interface.
type engine struct {
	FileStoreAPI *FileStoreAPI
	EventsChan   chan<- Event
	Users        map[string]*User
	Groups       map[string]*Group
	Mutex        *sync.RWMutex
}

//RunEngineAsync runs the main engine of portunus-server. It consumes the
//FileStoreAPI and returns an Engine interface for the HTTP server to use, and
//a stream of events for the LDAP worker.
func RunEngineAsync(fsAPI *FileStoreAPI) (Engine, <-chan Event) {
	eventsChan := make(chan Event)
	e := engine{
		FileStoreAPI: fsAPI,
		EventsChan:   eventsChan,
		Users:        make(map[string]*User),
		Groups:       make(map[string]*Group),
		Mutex:        &sync.RWMutex{},
	}

	go func() {
		for db := range e.FileStoreAPI.LoadEvents {
			e.handleLoadEvent(db)
		}
	}()

	return &e, eventsChan
}

func (e *engine) handleLoadEvent(db Database) {
	e.Mutex.Lock()
	defer e.Mutex.Unlock()

	var event Event
	keepUser := make(map[string]bool)
	keepGroup := make(map[string]bool)

	for _, userNew := range db.Users {
		keepUser[userNew.LoginName] = true
		userOld, exists := e.Users[userNew.LoginName]
		if exists {
			if !userOld.IsEqualTo(userNew) {
				mod := Modification{Old: userOld.connect(e), New: userNew.connect(e)}
				event.Modified = append(event.Modified, mod)
			}
		} else {
			event.Added = append(event.Added, userNew.connect(e))
		}
		clone := userNew
		e.Users[userNew.LoginName] = &clone
	}

	for _, groupNew := range db.Groups {
		keepGroup[groupNew.Name] = true
		groupOld, exists := e.Groups[groupNew.Name]
		if exists {
			if !groupOld.IsEqualTo(groupNew) {
				mod := Modification{Old: groupOld.connect(e), New: groupNew.connect(e)}
				event.Modified = append(event.Modified, mod)
			}
		} else {
			event.Added = append(event.Added, groupNew.connect(e))
		}
		clone := groupNew
		e.Groups[groupNew.Name] = &clone
	}

	for _, userOld := range e.Users {
		if keepUser[userOld.LoginName] {
			continue
		}
		event.Deleted = append(event.Deleted, userOld.connect(e))
		delete(e.Users, userOld.LoginName)
	}

	for _, groupOld := range e.Groups {
		if keepGroup[groupOld.Name] {
			continue
		}
		event.Deleted = append(event.Deleted, groupOld.connect(e))
		delete(e.Groups, groupOld.Name)
	}

	e.EventsChan <- event
}

//FindUser implements the Engine interface.
func (e *engine) FindUser(loginName string) *UserWithPerms {
	e.Mutex.RLock()
	defer e.Mutex.RUnlock()

	user, exists := e.Users[loginName]
	if !exists {
		return nil
	}

	curr := UserWithPerms{User: user.connect(e)}
	for _, group := range e.Groups {
		if group.ContainsUser(*user) {
			curr.GroupMemberships = append(curr.GroupMemberships, group.connect(e))
			curr.Perms = curr.Perms.Union(group.Permissions)
		}
	}
	return &curr
}

//ListGroups implements the Engine interface.
func (e *engine) ListGroups() []Group {
	e.Mutex.RLock()
	defer e.Mutex.RUnlock()

	var result []Group
	for _, group := range e.Groups {
		result = append(result, group.connect(e))
	}
	return result
}

//ListUsers implements the Engine interface.
func (e *engine) ListUsers() []User {
	e.Mutex.RLock()
	defer e.Mutex.RUnlock()

	var result []User
	for _, user := range e.Users {
		result = append(result, user.connect(e))
	}
	return result
}
