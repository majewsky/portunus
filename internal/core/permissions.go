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

// Permissions represents the permissions that membership in a certain group
// gives its members.
type Permissions struct {
	Portunus PortunusPermissions `json:"portunus"`
	LDAP     LDAPPermissions     `json:"ldap"`
}

// PortunusPermissions appears in type Permissions.
type PortunusPermissions struct {
	IsAdmin bool `json:"is_admin"`
}

// LDAPPermissions appears in type Permissions.
type LDAPPermissions struct {
	CanRead bool `json:"can_read"`
}

// Includes returns true when all the permissions are included in this
// Permissions instance.
func (p Permissions) Includes(other Permissions) bool {
	return p.Union(other) == p
}

// Union returns the union of the given permission sets.
func (p Permissions) Union(other Permissions) Permissions {
	var result Permissions
	result.Portunus.IsAdmin = p.Portunus.IsAdmin || other.Portunus.IsAdmin
	result.LDAP.CanRead = p.LDAP.CanRead || other.LDAP.CanRead
	return result
}
