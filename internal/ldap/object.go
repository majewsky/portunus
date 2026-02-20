/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package ldap

import (
	"fmt"

	"github.com/majewsky/portunus/internal/core"
)

// Object describes an object that can be stored in the LDAP directory.
type Object struct {
	DN         string
	Attributes map[string][]string
}

// Produces the LDAP objects representing the given group.
func renderGroup(g core.Group, dnSuffix string) []Object {
	memberDNames := make([]string, 0, len(g.MemberLoginNames))
	memberLoginNames := make([]string, 0, len(g.MemberLoginNames))
	for name, isMember := range g.MemberLoginNames {
		if isMember {
			memberDNames = append(memberDNames, fmt.Sprintf("uid=%s,ou=users,%s", name, dnSuffix))
			memberLoginNames = append(memberLoginNames, name)
		}
	}
	if len(memberDNames) == 0 {
		// The OpenLDAP core.schema requires that `groupOfNames` contain at least
		// one `member` attribute. If the group does not have any proper members,
		// add the dummy user account "nobody" to it.
		memberDNames = append(memberDNames, "cn=nobody,"+dnSuffix)
	}

	objs := []Object{{
		DN: fmt.Sprintf("cn=%s,ou=groups,%s", g.Name, dnSuffix),
		Attributes: map[string][]string{
			"cn":          {g.Name},
			"member":      memberDNames,
			"objectClass": {"groupOfNames", "top"},
		},
	}}
	if g.PosixGID != nil {
		objs = append(objs, Object{
			DN: fmt.Sprintf("cn=%s,ou=posix-groups,%s", g.Name, dnSuffix),
			Attributes: map[string][]string{
				"cn":          {g.Name},
				"gidNumber":   {g.PosixGID.String()},
				"memberUid":   memberLoginNames,
				"objectClass": {"posixGroup", "top"},
			},
		})
	}
	return objs
}

// Produces the LDAP object representing the given user.
func renderUser(u core.User, dnSuffix string, allGroups []core.Group) Object {
	var memberOfGroupDNames []string
	for _, group := range allGroups {
		if group.ContainsUser(u) {
			dn := fmt.Sprintf("cn=%s,ou=groups,%s", group.Name, dnSuffix)
			memberOfGroupDNames = append(memberOfGroupDNames, dn)
		}
	}

	obj := Object{
		DN: fmt.Sprintf("uid=%s,ou=users,%s", u.LoginName, dnSuffix),
		Attributes: map[string][]string{
			"uid":          {u.LoginName},
			"cn":           {u.FullName()},
			"sn":           {u.FamilyName},
			"givenName":    {u.GivenName},
			"userPassword": {u.PasswordHash},
			"isMemberOf":   memberOfGroupDNames,
			"objectClass":  {"portunusPerson", "inetOrgPerson", "organizationalPerson", "person", "top"},
		},
	}

	if u.EMailAddress != "" {
		obj.Attributes["mail"] = []string{u.EMailAddress}
	}
	if len(u.SSHPublicKeys) > 0 {
		obj.Attributes["sshPublicKey"] = u.SSHPublicKeys
	}

	if u.POSIX != nil {
		obj.Attributes["uidNumber"] = []string{u.POSIX.UID.String()}
		obj.Attributes["gidNumber"] = []string{u.POSIX.GID.String()}
		obj.Attributes["homeDirectory"] = []string{u.POSIX.HomeDirectory}
		if u.POSIX.LoginShell != "" {
			obj.Attributes["loginShell"] = []string{u.POSIX.LoginShell}
		}
		if u.POSIX.GECOS == "" {
			obj.Attributes["gecos"] = []string{u.FullName()}
		} else {
			obj.Attributes["gecos"] = []string{u.POSIX.GECOS}
		}
		obj.Attributes["objectClass"] = append(obj.Attributes["objectClass"], "posixAccount")
	}

	return obj
}
