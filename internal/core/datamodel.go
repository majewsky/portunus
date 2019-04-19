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

//Engine contains the interface that other parts of the application (e.g. the
//LDAP server or the web GUI) use to query the user/group database.
type Engine interface {
	//Subscription returns a sink that provides all updates to the data model.
	//The first event will contain the entire set of users and groups in its
	//AddedUsers and AddedGroups field.
	Subscribe() <-chan Event
}

//Event describes a change to an Engine's data model.
type Event struct {
	AddedUsers     []User
	AddedGroups    []Group
	ModifiedUsers  []User
	ModifiedGroups []Group
	DeletedUsers   []User
	DeletedGroups  []Group
}

//User represents a single user account.
type User struct {
	LoginName  string   `json:"login_name"`
	GivenName  string   `json:"given_name"`
	FamilyName string   `json:"family_name"`
	Password   Password `json:"password"`
}

//Password is a hashed password carrying metadata about the algorithm that was
//used to hash the password.
type Password struct {
	Algorithm string `json:"algo"`
	Hash      string `json:"hash"`
}

//Group represents a single group of users. Membership in a group implicitly
//grants its Permissions to all users in that group.
type Group struct {
	Name             string      `json:"name"`
	Description      string      `json:"description"`
	MemberLoginNames []string    `json:"members"`
	Permissions      Permissions `json:"permissions"`
}

//Permissions represents
type Permissions struct {
	LDAP LDAPAccessLevel `json:"ldap"`
}

//LDAPAccessLevel is an enum of permission levels for LDAP.
type LDAPAccessLevel string

const (
	//LDAPAccessNone is the access level for users that do not have access to
	//LDAP, i.e. bind requests will fail.
	LDAPAccessNone LDAPAccessLevel = ""
	//LDAPAccessFullRead allows users to read all entries in the LDAP directory.
	LDAPAccessFullRead = "full-read"
)
