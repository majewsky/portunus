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
	"fmt"
	"reflect"

	goldap "gopkg.in/ldap.v3"
)

//Group represents a single group of users. Membership in a group implicitly
//grants its Permissions to all users in that group.
type Group struct {
	Name             string      `json:"name"`
	LongName         string      `json:"long_name"`
	MemberLoginNames []string    `json:"members"`
	Permissions      Permissions `json:"permissions"`

	Engine Engine `json:"-"`
}

func (g Group) connect(e Engine) Group {
	g.Engine = e
	return g
}

//ContainsUser checks whether this group contains the given user.
func (g Group) ContainsUser(u User) bool {
	for _, name := range g.MemberLoginNames {
		if name == u.LoginName {
			return true
		}
	}
	return false
}

//IsEqualTo implements the Entity interface.
func (g Group) IsEqualTo(other Entity) bool {
	lhs := g
	rhs, ok := other.(Group)
	if !ok {
		return false
	}

	lhs.Engine = nil
	rhs.Engine = nil
	//cannot use `lhs == rhs` because of []string member
	return reflect.DeepEqual(lhs, rhs)
}

//RenderToLDAP implements the Entity interface.
func (g Group) RenderToLDAP(suffix string) goldap.AddRequest {
	//TODO: allow making this a posixGroup instead of a groupOfNames (requires gidNumber attribute)
	//NOTE: maybe duplicate posixGroups under a different ou so that we can have both a groupOfNames and a posixGroup for the same Group

	memberDNames := make([]string, len(g.MemberLoginNames))
	for idx, name := range g.MemberLoginNames {
		memberDNames[idx] = fmt.Sprintf("uid=%s,ou=users,%s", name, suffix)
	}

	return goldap.AddRequest{
		DN: fmt.Sprintf("cn=%s,ou=groups,%s", g.Name, suffix),
		Attributes: []goldap.Attribute{
			mkAttr("cn", g.Name),
			mkAttr("member", memberDNames...),
			mkAttr("objectClass", "groupOfNames", "top"),
		},
	}
}
