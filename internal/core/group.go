/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/majewsky/portunus/internal/grammars"
	"github.com/sapcc/go-bits/errext"
)

// Group represents a single group of users. Membership in a group implicitly
// grants its Permissions to all users in that group.
type Group struct {
	Name             string           `json:"name"`
	LongName         string           `json:"long_name"`
	MemberLoginNames GroupMemberNames `json:"members"`
	Permissions      Permissions      `json:"permissions"`
	PosixGID         *PosixID         `json:"posix_gid,omitempty"`
}

// Key implements the Object interface.
func (g Group) Key() string {
	return g.Name
}

// Cloned implements the Object interface.
func (g Group) Cloned() Group {
	logins := g.MemberLoginNames
	g.MemberLoginNames = make(GroupMemberNames)
	for name, isMember := range logins {
		if isMember {
			g.MemberLoginNames[name] = true
		}
	}
	if g.PosixGID != nil {
		val := *g.PosixGID
		g.PosixGID = &val
	}
	return g
}

// ContainsUser checks whether this group contains the given user.
func (g Group) ContainsUser(u User) bool {
	return g.MemberLoginNames[u.LoginName]
}

// GroupMemberNames is the type of Group.MemberLoginNames.
type GroupMemberNames map[string]bool

// MarshalJSON implements the json.Marshaler interface.
func (g GroupMemberNames) MarshalJSON() ([]byte, error) {
	names := make([]string, 0, len(g))
	for name, isMember := range g {
		if isMember {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return json.Marshal(names)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (g *GroupMemberNames) UnmarshalJSON(data []byte) error {
	var names []string
	err := json.Unmarshal(data, &names)
	if err != nil {
		return err
	}
	*g = make(map[string]bool)
	for _, name := range names {
		(*g)[name] = true
	}
	return nil
}

// Ref returns an ObjectRef that can be used to build validation errors.
func (g Group) Ref() ObjectRef {
	return ObjectRef{
		Type: "group",
		Name: g.Name,
	}
}

// Checks the individual attributes of this Group. Relationships and uniqueness
// are checked in Database.Validate().
func (g Group) validateLocal() (errs errext.ErrorSet) {
	ref := g.Ref()
	errs.Add(ref.Field("name").WrapFirst(
		MustNotBeEmpty(g.Name),
		MustNotHaveSurroundingSpaces(g.Name),
		MustBePosixAccountName(g.Name),
	))
	errs.Add(ref.Field("long_name").WrapFirst(
		MustNotBeEmpty(g.LongName),
		MustNotHaveSurroundingSpaces(g.LongName),
	))
	return
}

////////////////////////////////////////////////////////////////////////////////

// PosixID represents a POSIX user or group ID.
type PosixID uint16

func (id PosixID) String() string {
	return strconv.FormatUint(uint64(id), 10)
}

// ParsePosixID parses a PosixID from its decimal text representation.
// If the parse fails, a ValidationError is returned, using the provided FieldRef.
func ParsePosixID(input string, ref FieldRef) (PosixID, error) {
	input = strings.TrimSpace(input)
	if !grammars.IsNonnegativeInteger(input) {
		return 0, ref.Wrap(errNotDecimalNumber)
	}
	value, err := strconv.ParseUint(input, 10, 16)
	if err != nil {
		return 0, ref.Wrap(errNotPosixUIDorGID)
	}
	return PosixID(value), nil
}
