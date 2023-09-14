/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import (
	"context"
	"errors"
	"slices"
	"sync"
)

// Engine is the core engine of portunus-server.
type Engine interface { //TODO remove this type, fold Find/List methods into Nexus and have UI use it directly
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
	DBMutex sync.RWMutex
	DB      Database
	Nexus   Nexus
}

// NewEngine intializes an Engine connected to a nexus.
func NewEngine(ctx context.Context, nexus Nexus) Engine {
	e := &engine{Nexus: nexus}
	nexus.AddListener(ctx, func(db Database) {
		e.DBMutex.Lock()
		defer e.DBMutex.Unlock()
		e.DB = db
	})
	return e
}

// FindUser implements the Engine interface.
func (e *engine) FindGroup(name string) *Group {
	e.DBMutex.RLock()
	defer e.DBMutex.RUnlock()

	for _, g := range e.DB.Groups {
		if g.Name == name {
			g = g.Cloned()
			return &g
		}
	}
	return nil
}

// FindUser implements the Engine interface.
func (e *engine) FindUser(loginName string) *UserWithPerms {
	e.DBMutex.RLock()
	defer e.DBMutex.RUnlock()

	for _, u := range e.DB.Users {
		if u.LoginName == loginName {
			return e.collectUserPerms(u)
		}
	}
	return nil
}

// FindUserByEMail implements the Engine interface.
func (e *engine) FindUserByEMail(emailAddress string) *UserWithPerms {
	e.DBMutex.RLock()
	defer e.DBMutex.RUnlock()

	for _, u := range e.DB.Users {
		if u.EMailAddress != "" && u.EMailAddress == emailAddress {
			return e.collectUserPerms(u)
		}
	}
	return nil
}

func (e *engine) collectUserPerms(u User) *UserWithPerms {
	//NOTE: This is always called from functions that have locked e.DBMutex,
	//so we don't need to do it ourselves.
	user := UserWithPerms{User: u.Cloned()}
	for _, group := range e.DB.Groups {
		if group.ContainsUser(u) {
			user.GroupMemberships = append(user.GroupMemberships, group.Cloned())
			user.Perms = user.Perms.Union(group.Permissions)
		}
	}
	return &user
}

// ListGroups implements the Engine interface.
func (e *engine) ListGroups() []Group {
	e.DBMutex.RLock()
	defer e.DBMutex.RUnlock()

	result := make([]Group, 0, len(e.DB.Groups))
	for _, group := range e.DB.Groups {
		result = append(result, group.Cloned())
	}
	return result
}

// ListUsers implements the Engine interface.
func (e *engine) ListUsers() []User {
	e.DBMutex.RLock()
	defer e.DBMutex.RUnlock()

	result := make([]User, 0, len(e.DB.Users))
	for _, user := range e.DB.Users {
		result = append(result, user.Cloned())
	}
	return result
}

// ChangeUser implements the Engine interface.
func (e *engine) ChangeUser(loginName string, action func(User) (*User, error)) error {
	reducer := func(db *Database) error {
		//update or delete existing user...
		for idx, user := range db.Users {
			if user.LoginName == loginName {
				newUser, err := action(user)
				if newUser == nil {
					db.Users = slices.Delete(db.Users, idx, idx+1)
				} else {
					db.Users[idx] = *newUser
				}
				return err
			}
		}

		//...or create new user
		newUser, err := action(User{})
		if newUser != nil {
			db.Users = append(db.Users, *newUser)
		}
		return err
	}

	errs := e.Nexus.Update(reducer, &UpdateOptions{ConflictWithSeedIsError: true})
	if errs.IsEmpty() {
		return nil
	}
	return errors.New(errs.Join(", "))
}

// ChangeGroup implements the Engine interface.
func (e *engine) ChangeGroup(name string, action func(Group) (*Group, error)) error {
	reducer := func(db *Database) error {
		//update or delete existing group...
		for idx, group := range db.Groups {
			if group.Name == name {
				newGroup, err := action(group)
				if newGroup == nil {
					db.Groups = slices.Delete(db.Groups, idx, idx+1)
				} else {
					db.Groups[idx] = *newGroup
				}
				return err
			}
		}

		//...or create new group
		newGroup, err := action(Group{})
		if newGroup != nil {
			db.Groups = append(db.Groups, *newGroup)
		}
		return err
	}

	errs := e.Nexus.Update(reducer, &UpdateOptions{ConflictWithSeedIsError: true})
	if errs.IsEmpty() {
		return nil
	}
	return errors.New(errs.Join(", "))
}
