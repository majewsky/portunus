/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/sapcc/go-bits/logg"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

// LDAPObject describes an object that can be stored in the LDAP directory.
type LDAPObject struct {
	DN         string
	Attributes map[string][]string
}

// Engine is the core engine of portunus-server.
type Engine interface {
	FindGroup(name string) *Group
	FindUser(loginName string) *UserWithPerms
	FindUserByEMail(emailAddress string) *UserWithPerms
	ListGroups() []Group
	ListUsers() []User
	//The ChangeX() methods are used to create, modify and delete entities.
	//When creating a new entity, the action is invoked with a
	//default-constructed argument. To delete an entity, return nil from the
	//action. If a non-nil error is returned, it's the one returned by the
	//action.
	ChangeUser(loginName string, action func(User) (*User, error)) error
	ChangeGroup(name string, action func(Group) (*Group, error)) error
}

// engine implements the Engine interface.
type engine struct {
	FileStoreAPI *FileStoreAPI
	Seed         *DatabaseSeed
	LDAPUpdates  chan<- []LDAPObject
	Users        map[string]User
	Groups       map[string]Group
	Mutex        *sync.RWMutex
	LDAPSuffix   string
}

// RunEngineAsync runs the main engine of portunus-server. It consumes the
// FileStoreAPI and returns an Engine interface for the HTTP server to use, and
// a stream of events for the LDAP worker.
func RunEngineAsync(fsAPI *FileStoreAPI, ldapSuffix string, seed *DatabaseSeed) (Engine, <-chan []LDAPObject) {
	ldapUpdatesChan := make(chan []LDAPObject, 1)
	e := engine{
		FileStoreAPI: fsAPI,
		Seed:         seed,
		LDAPUpdates:  ldapUpdatesChan,
		Mutex:        &sync.RWMutex{},
		LDAPSuffix:   ldapSuffix,
	}

	go func() {
		for db := range e.FileStoreAPI.LoadEvents {
			e.handleLoadEvent(db)
		}
	}()

	return &e, ldapUpdatesChan
}

func (e *engine) findGroupSeed(name string) *GroupSeed {
	if e.Seed == nil {
		return nil
	}
	for _, g := range e.Seed.Groups {
		if string(g.Name) == name {
			return &g
		}
	}
	return nil
}

func (e *engine) findUserSeed(loginName string) *UserSeed {
	if e.Seed == nil {
		return nil
	}
	for _, u := range e.Seed.Users {
		if string(u.LoginName) == loginName {
			return &u
		}
	}
	return nil
}

func (e *engine) handleLoadEvent(db Database) {
	e.Mutex.Lock()
	defer e.Mutex.Unlock()

	seedApplied := false

	e.Groups = make(map[string]Group, len(db.Groups))
	for _, group := range db.Groups {
		e.Groups[group.Name] = group

		//check if seed needs to be re-applied
		groupSeed := e.findGroupSeed(group.Name)
		if groupSeed != nil {
			groupCloned := group.Cloned()
			groupSeed.ApplyTo(&groupCloned)
			if !reflect.DeepEqual(group, groupCloned) {
				e.Groups[group.Name] = groupCloned
				seedApplied = true
			}
		}
	}

	e.Users = make(map[string]User, len(db.Users))
	for _, user := range db.Users {
		e.Users[user.LoginName] = user

		//check if seed needs to be re-applied
		userSeed := e.findUserSeed(user.LoginName)
		if userSeed != nil {
			userCloned := user.Cloned()
			userSeed.ApplyTo(&userCloned)
			if !reflect.DeepEqual(user, userCloned) {
				e.Users[user.LoginName] = userCloned
				seedApplied = true
			}
		}
	}

	// create in db missing seeded users
	knownUsers := maps.Keys(e.Users)
	for _, userSeed := range e.Seed.Users {
		if !slices.Contains(knownUsers, string(userSeed.LoginName)) {
			user := User{
				LoginName: string(userSeed.LoginName),
			}
			userSeed.ApplyTo(&user)
			e.Users[string(userSeed.LoginName)] = user
			seedApplied = true
		}
	}

	// create in db missing seeded groups
	knownGroups := maps.Keys(e.Groups)
	for _, seedGroup := range e.Seed.Groups {
		if !slices.Contains(knownGroups, string(seedGroup.Name)) {
			group := Group{
				Name: string(seedGroup.Name),
			}
			seedGroup.ApplyTo(&group)
			e.Groups[string(seedGroup.Name)] = group
			seedApplied = true
		}
	}

	if seedApplied {
		e.persistDatabase()
	}
	e.persistLDAP()
}

// FindUser implements the Engine interface.
func (e *engine) FindGroup(name string) *Group {
	e.Mutex.RLock()
	defer e.Mutex.RUnlock()

	g, exists := e.Groups[name]
	if !exists {
		return nil
	}
	g = g.Cloned()
	return &g
}

// FindUser implements the Engine interface.
func (e *engine) FindUser(loginName string) *UserWithPerms {
	e.Mutex.RLock()
	defer e.Mutex.RUnlock()

	u, exists := e.Users[loginName]
	if !exists {
		return nil
	}
	return e.collectUserPerms(u)
}

// FindUserByEMail implements the Engine interface.
func (e *engine) FindUserByEMail(emailAddress string) *UserWithPerms {
	e.Mutex.RLock()
	defer e.Mutex.RUnlock()

	for _, u := range e.Users {
		if u.EMailAddress != "" && u.EMailAddress == emailAddress {
			return e.collectUserPerms(u)
		}
	}
	return nil
}

func (e *engine) collectUserPerms(u User) *UserWithPerms {
	//NOTE: This is always called from functions that have locked e.Mutex, so we
	//don't need to do it ourselves.
	user := UserWithPerms{User: u.Cloned()}
	for _, group := range e.Groups {
		if group.ContainsUser(u) {
			user.GroupMemberships = append(user.GroupMemberships, group.Cloned())
			user.Perms = user.Perms.Union(group.Permissions)
		}
	}
	return &user
}

// ListGroups implements the Engine interface.
func (e *engine) ListGroups() []Group {
	e.Mutex.RLock()
	defer e.Mutex.RUnlock()

	result := make([]Group, 0, len(e.Groups))
	for _, group := range e.Groups {
		result = append(result, group.Cloned())
	}
	return result
}

// ListUsers implements the Engine interface.
func (e *engine) ListUsers() []User {
	e.Mutex.RLock()
	defer e.Mutex.RUnlock()

	result := make([]User, 0, len(e.Users))
	for _, user := range e.Users {
		result = append(result, user.Cloned())
	}
	return result
}

var (
	errCannotDeleteSeededGroup         = errors.New("cannot delete group that is statically configured in seed")
	errCannotOverwriteSeededGroupAttrs = errors.New("cannot overwrite group attributes that are statically configured in seed")
	errCannotDeleteSeededUser          = errors.New("cannot delete user account that is statically configured in seed")
	errCannotOverwriteSeededUserAttrs  = errors.New("cannot overwrite user attributes that are statically configured in seed")
)

// ChangeUser implements the Engine interface.
func (e *engine) ChangeUser(loginName string, action func(User) (*User, error)) error {
	e.Mutex.Lock()
	defer e.Mutex.Unlock()

	oldState, exists := e.Users[loginName]
	oldStatePtr := &oldState
	if !exists {
		oldStatePtr = nil
	}
	newState, err := action(oldState.Cloned())
	if err != nil {
		return err
	}

	//check that changed user still conforms with seed (if any)
	userSeed := e.findUserSeed(loginName)
	if userSeed != nil {
		if newState == nil {
			return errCannotDeleteSeededUser
		}
		newStateCloned := newState.Cloned()
		userSeed.ApplyTo(&newStateCloned)
		if !reflect.DeepEqual(newStateCloned, *newState) {
			logg.Debug("seed check failed: newState before seed = %#v", *newState)
			logg.Debug("seed check failed: newState after seed  = %#v", newStateCloned)
			return errCannotOverwriteSeededUserAttrs
		}
	}

	//only change database if there are actual changes
	if newState == nil {
		if oldStatePtr == nil {
			return nil
		}
		delete(e.Users, loginName)
	} else {
		if reflect.DeepEqual(oldState, *newState) {
			return nil
		}
		e.Users[loginName] = *newState
	}

	e.persistDatabase()
	e.persistLDAP()
	return nil
}

// ChangeGroup implements the Engine interface.
func (e *engine) ChangeGroup(name string, action func(Group) (*Group, error)) error {
	e.Mutex.Lock()
	defer e.Mutex.Unlock()

	oldState, exists := e.Groups[name]
	oldStatePtr := &oldState
	if !exists {
		oldStatePtr = nil
	}
	newState, err := action(oldState.Cloned())
	if err != nil {
		return err
	}

	//check that changed group still conforms with seed (if any)
	groupSeed := e.findGroupSeed(name)
	if groupSeed != nil {
		if newState == nil {
			return errCannotDeleteSeededGroup
		}
		newStateCloned := newState.Cloned()
		newStateWithSeedApplied := newState.Cloned()
		groupSeed.ApplyTo(&newStateWithSeedApplied)
		if !reflect.DeepEqual(newStateCloned, newStateWithSeedApplied) {
			// NOTE: This uses `newState.Cloned()` instead of `*newState` as the LHS
			// to normalize MemberLoginNames.
			logg.Debug("seed check failed: newState before seed = %#v", newStateCloned)
			logg.Debug("seed check failed: newState after seed  = %#v", newStateWithSeedApplied)
			return errCannotOverwriteSeededGroupAttrs
		}
	}

	//only change database if there are actual changes
	if newState == nil {
		if oldStatePtr == nil {
			return nil
		}
		delete(e.Groups, name)
	} else {
		if oldState.IsEqualTo(*newState) {
			return nil
		}
		e.Groups[name] = *newState
	}

	e.persistDatabase()
	e.persistLDAP()
	return nil
}

func (e *engine) persistDatabase() {
	//NOTE: This is always called from functions that have locked e.Mutex, so we
	//don't need to do it ourselves.
	var db Database
	for _, user := range e.Users {
		db.Users = append(db.Users, user.Cloned())
	}
	for _, group := range e.Groups {
		db.Groups = append(db.Groups, group.Cloned())
	}
	e.FileStoreAPI.SaveRequests <- db
}

func (e *engine) persistLDAP() {
	//NOTE: This is always called from functions that have locked e.Mutex, so we
	//don't need to do it ourselves.
	ldapDB := make([]LDAPObject, 0, len(e.Users)+len(e.Groups)+1)
	for _, user := range e.Users {
		ldapDB = append(ldapDB, user.RenderToLDAP(e.LDAPSuffix, e.Groups))
	}
	for _, group := range e.Groups {
		ldapDB = append(ldapDB, group.RenderToLDAP(e.LDAPSuffix)...)
	}
	ldapDB = append(ldapDB, e.renderVirtualGroups()...)
	e.LDAPUpdates <- ldapDB
}

func (e *engine) renderVirtualGroups() []LDAPObject {
	//NOTE: This is always called from functions that have locked e.Mutex, so we
	//don't need to do it ourselves.
	isLDAPViewerDN := make(map[string]bool)
	for _, group := range e.Groups {
		if group.Permissions.LDAP.CanRead {
			for loginName, isMember := range group.MemberLoginNames {
				if isMember {
					dn := fmt.Sprintf("uid=%s,ou=users,%s", loginName, e.LDAPSuffix)
					isLDAPViewerDN[dn] = true
				}
			}
		}
	}
	ldapViewerDNames := make([]string, 0, len(isLDAPViewerDN))
	for dn := range isLDAPViewerDN {
		ldapViewerDNames = append(ldapViewerDNames, dn)
	}
	if len(ldapViewerDNames) == 0 {
		ldapViewerDNames = append(ldapViewerDNames, "cn=nobody,"+e.LDAPSuffix)
	}

	return []LDAPObject{{
		DN: fmt.Sprintf("cn=portunus-viewers,%s", e.LDAPSuffix),
		Attributes: map[string][]string{
			"cn":          {"portunus-viewers"},
			"member":      ldapViewerDNames,
			"objectClass": {"groupOfNames", "top"},
		},
	}}
}
