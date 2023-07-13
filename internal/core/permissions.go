/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
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
