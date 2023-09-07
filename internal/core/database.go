/*******************************************************************************
* Copyright 2019-2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import (
	"fmt"
	"sort"

	"github.com/sapcc/go-bits/errext"
)

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

// Validate checks all users and groups in this Database for validity.
func (d Database) Validate() (errs errext.ErrorSet) {
	//check user attributes
	userCount := make(map[string]uint)
	for _, u := range d.Users {
		errs.Append(u.validateLocal())
		userCount[u.LoginName]++
	}

	//check group attributes and membership
	groupCount := make(map[string]uint)
	for _, g := range d.Groups {
		errs.Append(g.validateLocal())
		groupCount[g.Name]++

		for loginName := range g.MemberLoginNames {
			if userCount[loginName] == 0 {
				err := fmt.Errorf("contains unknown user with login name %q", loginName)
				errs.Add(ValidationError{g.FieldRef("members"), err})
			}
		}
	}

	//check user name uniqueness
	for loginName, count := range userCount {
		if count > 1 {
			ref := User{LoginName: loginName}.FieldRef("login_name")
			errs.Add(ref.Wrap(errIsDuplicate))
		}
	}

	//check group name uniqueness
	for name, count := range groupCount {
		if count > 1 {
			ref := Group{Name: name}.FieldRef("name")
			errs.Add(ref.Wrap(errIsDuplicate))
		}
	}

	return
}
