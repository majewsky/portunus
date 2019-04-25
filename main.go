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

package main

import (
	"github.com/majewsky/portunus/internal/core"
	"github.com/majewsky/portunus/internal/ldap"
	"github.com/sapcc/go-bits/logg"
)

func main() {
	logg.ShowDebug = true

	//run LDAP engine
	engine := mockEngine{}
	eventsChan := engine.Subscribe()
	go ldap.RunServer(eventsChan)

	for {
		select {}
	}
}

type mockEngine struct{}

//Subscribe implements the core.Engine interface.
func (mockEngine) Subscribe() <-chan core.Event {
	users := []core.User{
		{
			LoginName:    "john",
			GivenName:    "John",
			FamilyName:   "Doe",
			PasswordHash: core.HashPasswordForLDAP("12345"),
		},
		{
			LoginName:    "jane",
			GivenName:    "Jane",
			FamilyName:   "Doe",
			PasswordHash: core.HashPasswordForLDAP("password"),
		},
	}
	groups := []core.Group{
		{
			Name:             "admins",
			Description:      "system administrators",
			MemberLoginNames: []string{"jane"},
			Permissions: core.Permissions{
				LDAP: core.LDAPAccessFullRead,
			},
		},
		{
			Name:             "users",
			Description:      "contains everyone",
			MemberLoginNames: []string{"jane", "john"},
			Permissions: core.Permissions{
				LDAP: core.LDAPAccessNone,
			},
		},
	}

	channel := make(chan core.Event, 10)
	channel <- core.Event{
		AddedUsers:  users,
		AddedGroups: groups,
	}
	return channel
}
