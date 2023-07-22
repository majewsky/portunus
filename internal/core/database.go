/*******************************************************************************
* Copyright 2019-2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import "sort"

// Database contains the contents of Portunus' database.
type Database struct {
	Users  []User
	Groups []Group
}

// Cloned returns a deep copy of this database.
func (d Database) Cloned() Database {
	result := Database{
		Users:  make([]User, len(d.Users)),
		Groups: make([]Group, len(d.Groups)),
	}
	for idx, u := range d.Users {
		result.Users[idx] = u.Cloned()
	}
	for idx, g := range d.Groups {
		result.Groups[idx] = g.Cloned()
	}
	return result
}

// FindGroup returns a copy of the first group matching the given predicate. If
// no matching group is found, false is returned in the second return value
// (like for a two-valued map lookup).
func (d Database) FindGroup(predicate func(Group) bool) (Group, bool) {
	for _, g := range d.Groups {
		if predicate(g) {
			return g.Cloned(), true
		}
	}
	return Group{}, false
}

// FindUser returns a copy of the first user matching the given predicate. If
// no matching user is found, false is returned in the second return value
// (like for a two-valued map lookup).
func (d Database) FindUser(predicate func(User) bool) (User, bool) {
	for _, u := range d.Users {
		if predicate(u) {
			return u.Cloned(), true
		}
	}
	return User{}, false
}

// IsEmpty returns whether this Database is zero-initialized.
func (d Database) IsEmpty() bool {
	return len(d.Users) == 0 && len(d.Groups) == 0
}

// Normalize applies idempotent transformations to this database to ensure
// stable comparison and serialization.
func (d *Database) Normalize() {
	for _, g := range d.Groups {
		for name, isMember := range g.MemberLoginNames {
			if !isMember {
				delete(g.MemberLoginNames, name)
			}
		}
	}

	sort.Slice(d.Groups, func(i, j int) bool {
		return d.Groups[i].Name < d.Groups[j].Name
	})
	sort.Slice(d.Users, func(i, j int) bool {
		return d.Users[i].LoginName < d.Users[j].LoginName
	})
}
