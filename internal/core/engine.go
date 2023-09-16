/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import (
	"context"
	"errors"
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

	group, exists := e.DB.Groups.Find(func(g Group) bool { return g.Name == name })
	if exists {
		g := group.Cloned()
		return &g
	}
	return nil
}

// FindUser implements the Engine interface.
func (e *engine) FindUser(loginName string) *UserWithPerms {
	e.DBMutex.RLock()
	defer e.DBMutex.RUnlock()

	user, exists := e.DB.Users.Find(func(u User) bool { return u.LoginName == loginName })
	if exists {
		return e.collectUserPerms(user)
	}
	return nil
}

// FindUserByEMail implements the Engine interface.
func (e *engine) FindUserByEMail(emailAddress string) *UserWithPerms {
	e.DBMutex.RLock()
	defer e.DBMutex.RUnlock()

	user, exists := e.DB.Users.Find(func(u User) bool {
		return u.EMailAddress != "" && u.EMailAddress == emailAddress
	})
	if exists {
		return e.collectUserPerms(user)
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
		oldUser, exists := db.Users.Find(func(u User) bool { return u.LoginName == loginName })
		if !exists {
			oldUser = User{}
		}
		newUser, err := action(oldUser)
		if newUser == nil {
			db.Users.Delete(loginName)
		} else {
			db.Users.InsertOrUpdate(*newUser)
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
		oldGroup, exists := db.Groups.Find(func(g Group) bool { return g.Name == name })
		if !exists {
			oldGroup = Group{}
		}
		newGroup, err := action(oldGroup)
		if newGroup == nil {
			db.Groups.Delete(name)
		} else {
			db.Groups.InsertOrUpdate(*newGroup)
		}
		return err
	}

	errs := e.Nexus.Update(reducer, &UpdateOptions{ConflictWithSeedIsError: true})
	if errs.IsEmpty() {
		return nil
	}
	return errors.New(errs.Join(", "))
}
