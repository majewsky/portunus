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

import goldap "gopkg.in/ldap.v3"

//Permissions represents the permissions that membership in a certain group
//gives its members.
type Permissions struct {
	LDAP LDAPAccessLevel `json:"ldap"`
}

//LDAPAccessLevel is an enum of permission levels for LDAP.
//TODO This is pathetic and needs to be way more granular.
type LDAPAccessLevel string

const (
	//LDAPAccessNone is the access level for users that do not have access to
	//LDAP, i.e. bind requests will fail.
	LDAPAccessNone LDAPAccessLevel = ""
	//LDAPAccessFullRead allows users to read all entries in the LDAP directory.
	LDAPAccessFullRead = "full-read"
)

func mkAttr(typeName string, values ...string) goldap.Attribute {
	return goldap.Attribute{Type: typeName, Vals: values}
}
