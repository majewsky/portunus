/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import (
	"context"
	"errors"
	"reflect"
	"sync"

	"github.com/majewsky/portunus/internal/crypt"
	"github.com/sapcc/go-bits/errext"
)

// UpdateAction is an action that modifies the contents of the Database.
// This type appears in the Nexus.Update() interface method.
type UpdateAction func(*Database) errext.ErrorSet

// Nexus stores the contents of the Database. All other parts of the
// application use a reference to the Nexus to read and update the Database.
type Nexus interface {
	// AddListener registers a listener with the nexus. Whenever the database
	// changes, the callback will be invoked to provide a copy of the database to
	// the listener. The listener will be removed from the nexus when `ctx`
	// expires.
	//
	// Note that the callback is invoked from whatever goroutine is causing the
	// DB to be updated. If a specific goroutine shall process the event, the
	// callback should send into a channel from which that goroutine is receiving.
	AddListener(ctx context.Context, callback func(Database))

	// Update changes the contents of the database. This interface follows the
	// State Reducer pattern: The action callback is invoked with the current
	// Database, and is expected to return the updated Database. The updated
	// Database is then validated and the database seed is enforced, if any.
	Update(action UpdateAction, opts *UpdateOptions) errext.ErrorSet

	// Assorted querying functions. The return values are always deep clones
	// of their respective database entries.
	ListGroups() []Group
	ListUsers() []User
	FindGroup(predicate func(Group) bool) (Group, bool)
	FindUser(predicate func(User) bool) (UserWithPerms, bool)

	// Components carried by the Nexus.
	PasswordHasher() crypt.PasswordHasher
}

// UpdateOptions controls optional behavior in Nexus.Update().
type UpdateOptions struct {
	//If true, conflicts with the seed will be reported as validation errors.
	//If false (default), conflicts with the seed will be corrected silently.
	ConflictWithSeedIsError bool

	//If true, the updated database will be computed and validated, but not
	//saved. This is used to obtain a more complete set of errors for the UI
	//after a preliminary validation step already failed.
	DryRun bool
}

// ErrDatabaseNeedsInitialization is used by the disk store connection to
// signal to Nexus.Update() that no disk store exists yet. It will prompt the
// nexus to perform first-time setup of the database contents.
var ErrDatabaseNeedsInitialization = errors.New("ErrDatabaseNeedsInitialization")

// NewNexus instantiates the Nexus.
func NewNexus(d *DatabaseSeed, cfg *ValidationConfig, hasher crypt.PasswordHasher) Nexus {
	return &nexusImpl{hasher: hasher, vcfg: cfg, seed: d}
}

type nexusImpl struct {
	hasher crypt.PasswordHasher
	vcfg   *ValidationConfig
	//The mutex guards access to all fields listed below it in this struct.
	mutex     sync.RWMutex
	seed      *DatabaseSeed
	db        Database
	listeners []listener
}

type listener struct {
	ctx      context.Context
	callback func(Database)
}

// PasswordHasher implements the Nexus interface.
func (n *nexusImpl) PasswordHasher() crypt.PasswordHasher {
	return n.hasher
}

// ListGroups implements the Nexus interface.
func (n *nexusImpl) ListGroups() []Group {
	n.mutex.RLock()
	defer n.mutex.RUnlock()
	return n.db.Groups.Cloned()
}

// ListUsers implements the Nexus interface.
func (n *nexusImpl) ListUsers() []User {
	n.mutex.RLock()
	defer n.mutex.RUnlock()
	return n.db.Users.Cloned()
}

// FindGroup implements the Nexus interface.
func (n *nexusImpl) FindGroup(predicate func(Group) bool) (Group, bool) {
	n.mutex.RLock()
	defer n.mutex.RUnlock()
	return n.db.Groups.Find(predicate)
}

// FindUser implements the Nexus interface.
func (n *nexusImpl) FindUser(predicate func(User) bool) (UserWithPerms, bool) {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	user, exists := n.db.Users.Find(predicate)
	if exists {
		return n.db.collectUserPermissions(user), true
	}
	return UserWithPerms{}, false
}

// AddListener implements the Nexus interface.
func (n *nexusImpl) AddListener(ctx context.Context, callback func(Database)) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	n.listeners = append(n.listeners, listener{ctx, callback})

	//if the DB has already been filled before AddListener(), tell the listener
	//about the current DB contents right away
	if !n.db.IsEmpty() && ctx.Err() == nil {
		callback(n.db.Cloned())
	}
}

// Update implements the Nexus interface.
func (n *nexusImpl) Update(action UpdateAction, optsPtr *UpdateOptions) (errs errext.ErrorSet) {
	var opts UpdateOptions
	if optsPtr != nil {
		opts = *optsPtr
	}

	n.mutex.Lock()
	defer n.mutex.Unlock()

	//compute new DB by applying the reducer to a clone of the old DB
	newDB := n.db.Cloned()
	errs = action(&newDB)
	if len(errs) == 1 && errs[0] == ErrDatabaseNeedsInitialization {
		newDB = initializeDatabase(n.seed, n.hasher)
		errs = nil
	}
	//^ NOTE: We do not return early on error here. For interactive updates,
	//we want to report as many errors as possible in a single go,
	//without differentiating between errors from the UpdateAction
	//and validation errors from the core logic.
	//
	//This is important for a consistent user experience because some
	//validation errors cannot be generated in the core and must come
	//from the UpdateAction (e.g. any checks involving unhashed passwords).

	//normalize the DB and validate it against common rules and the seed
	newDB.Normalize()
	errs.Append(newDB.Validate(n.vcfg))
	if n.seed != nil {
		if opts.ConflictWithSeedIsError {
			errs.Append(n.seed.CheckConflicts(newDB, n.hasher))
		} else {
			n.seed.ApplyTo(&newDB, n.hasher)
		}
	}

	//do we have a reason to not update the DB for real?
	if opts.DryRun || !errs.IsEmpty() {
		return errs
	}

	//new DB looks good -> store it and inform our listeners *if* it actually
	//represents a change
	if reflect.DeepEqual(n.db, newDB) {
		return nil
	}
	n.db = newDB
	for _, listener := range n.listeners {
		if listener.ctx.Err() == nil {
			listener.callback(n.db.Cloned())
		}
	}
	return nil
}
