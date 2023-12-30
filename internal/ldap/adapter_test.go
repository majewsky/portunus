/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package ldap

import (
	"context"
	"sync"
	"testing"
	"time"

	goldap "github.com/go-ldap/ldap/v3"
	"github.com/majewsky/portunus/internal/core"
	"github.com/majewsky/portunus/internal/test"
	"github.com/sapcc/go-bits/errext"
)

func setupAdapterTest(t *testing.T) (conn *test.LDAPConnectionDouble, updateDBWithRunningAdapter func(core.UpdateAction) errext.ErrorSet) {
	vcfg := core.GetValidationConfigForTests()
	nexus := core.NewNexus(nil, vcfg, &core.NoopHasher{})
	conn = test.NewLDAPConnectionDouble("dc=example,dc=org")
	adapter := NewAdapter(nexus, conn)

	//This can be used by the test to update the database while adapter.Run() is
	//running in a separate goroutine. This function takes care to shutdown
	//adapter.Run() before it returns, thus ensuring that all effects of the
	//action on the LDAPConnectionDouble have been observed.
	updateDBWithRunningAdapter = func(action core.UpdateAction) errext.ErrorSet {
		//This log is a breadcrumb for associating a failed adapter.Run() with the
		//respective updateDBWithRunningAdapter() callsite. We cannot use the
		//default callsite attribution through a goroutine boundary.
		t.Helper()
		t.Log("updateDBWithRunningAdapter running")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			test.ExpectNoError(t, adapter.Run(ctx))
		}()

		errs := nexus.Update(action, nil)
		time.Sleep(10 * time.Millisecond) //give the Adapter some time to complete outstanding actions
		cancel()
		wg.Wait()
		return errs
	}

	//When adapter.Run() is first executed, we will observe the following
	//requests to build the topmost part of the object hierarchy.
	conn.ExpectAdd(goldap.AddRequest{
		DN: "dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "dc", Vals: []string{"example"}},
			{Type: "o", Vals: []string{"example"}},
			{Type: "objectClass", Vals: []string{"dcObject", "organization", "top"}},
		},
	})
	conn.ExpectAdd(goldap.AddRequest{
		DN: "ou=users,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "ou", Vals: []string{"users"}},
			{Type: "objectClass", Vals: []string{"organizationalUnit", "top"}},
		},
	})
	conn.ExpectAdd(goldap.AddRequest{
		DN: "ou=groups,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "ou", Vals: []string{"groups"}},
			{Type: "objectClass", Vals: []string{"organizationalUnit", "top"}},
		},
	})
	conn.ExpectAdd(goldap.AddRequest{
		DN: "ou=posix-groups,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "ou", Vals: []string{"posix-groups"}},
			{Type: "objectClass", Vals: []string{"organizationalUnit", "top"}},
		},
	})
	conn.ExpectAdd(goldap.AddRequest{
		DN: "cn=portunus,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "cn", Vals: []string{"portunus"}},
			{Type: "description", Vals: []string{"Internal service user for Portunus"}},
			{Type: "objectClass", Vals: []string{"organizationalRole", "top"}},
		},
	})
	conn.ExpectAdd(goldap.AddRequest{
		DN: "cn=nobody,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "cn", Vals: []string{"nobody"}},
			{Type: "description", Vals: []string{"Dummy user for empty groups (all groups need to have at least one member)"}},
			{Type: "objectClass", Vals: []string{"organizationalRole", "top"}},
		},
	})

	return
}

func TestBasicOperations(t *testing.T) {
	//The focus of this test is just to see that basic add/modify/delete
	//operations work. If possible, please add a new test instead of extending
	//this one.
	conn, updateDBWithRunningAdapter := setupAdapterTest(t)

	//when we add a user and group...
	action := func(db *core.Database) errext.ErrorSet {
		db.Users = []core.User{{
			LoginName:    "alice",
			GivenName:    "Alice",
			FamilyName:   "Administrator",
			PasswordHash: "x",
		}}
		db.Groups = []core.Group{{
			Name:     "grafana-users",
			LongName: "We monitor the monitoring.",
		}}
		return nil
	}

	//...one LDAP object should be created for both of them (also, since this is
	//our first actual DB update, the "portunus-viewers" group is created empty)
	conn.ExpectAdd(goldap.AddRequest{
		DN: "uid=alice,ou=users,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "uid", Vals: []string{"alice"}},
			{Type: "cn", Vals: []string{"Alice Administrator"}},
			{Type: "sn", Vals: []string{"Administrator"}},
			{Type: "givenName", Vals: []string{"Alice"}},
			{Type: "userPassword", Vals: []string{"x"}},
			{Type: "objectClass", Vals: []string{"portunusPerson", "inetOrgPerson", "organizationalPerson", "person", "top"}},
		},
	})
	conn.ExpectAdd(goldap.AddRequest{
		DN: "cn=grafana-users,ou=groups,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "cn", Vals: []string{"grafana-users"}},
			{Type: "member", Vals: []string{"cn=nobody,dc=example,dc=org"}}, //placeholder because attribute is required
			{Type: "objectClass", Vals: []string{"groupOfNames", "top"}},
		},
	})
	conn.ExpectAdd(goldap.AddRequest{
		DN: "cn=portunus-viewers,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "cn", Vals: []string{"portunus-viewers"}},
			{Type: "member", Vals: []string{"cn=nobody,dc=example,dc=org"}}, //placeholder because attribute is required
			{Type: "objectClass", Vals: []string{"groupOfNames", "top"}},
		},
	})
	test.ExpectNoErrors(t, updateDBWithRunningAdapter(action))
	conn.CheckAllExecuted(t)

	//there is no rename operation: when we change the RDN of an object...
	action = func(db *core.Database) errext.ErrorSet {
		db.Groups[0].Name = "grafana-admins"
		return nil
	}

	//...the old object is deleted and a new object is created
	conn.ExpectDelete(goldap.DelRequest{
		DN: "cn=grafana-users,ou=groups,dc=example,dc=org",
	})
	conn.ExpectAdd(goldap.AddRequest{
		DN: "cn=grafana-admins,ou=groups,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "cn", Vals: []string{"grafana-admins"}},
			{Type: "member", Vals: []string{"cn=nobody,dc=example,dc=org"}}, //placeholder because attribute is required
			{Type: "objectClass", Vals: []string{"groupOfNames", "top"}},
		},
	})
	test.ExpectNoErrors(t, updateDBWithRunningAdapter(action))
	conn.CheckAllExecuted(t)

	//we can test object updates by adding the user to the group...
	action = func(db *core.Database) errext.ErrorSet {
		db.Groups[0].MemberLoginNames = core.GroupMemberNames{db.Users[0].LoginName: true}
		return nil
	}

	//...this should modify the member references on both the user and the group
	conn.ExpectModify(goldap.ModifyRequest{
		DN: "cn=grafana-admins,ou=groups,dc=example,dc=org",
		Changes: []goldap.Change{{
			Operation:    goldap.ReplaceAttribute,
			Modification: goldap.PartialAttribute{Type: "member", Vals: []string{"uid=alice,ou=users,dc=example,dc=org"}},
		}},
	})
	conn.ExpectModify(goldap.ModifyRequest{
		DN: "uid=alice,ou=users,dc=example,dc=org",
		Changes: []goldap.Change{{
			Operation:    goldap.ReplaceAttribute,
			Modification: goldap.PartialAttribute{Type: "isMemberOf", Vals: []string{"cn=grafana-admins,ou=groups,dc=example,dc=org"}},
		}},
	})
	test.ExpectNoErrors(t, updateDBWithRunningAdapter(action))
	conn.CheckAllExecuted(t)

	//by deleting the group...
	action = func(db *core.Database) errext.ErrorSet {
		db.Groups = nil
		return nil
	}

	//...we can test both object deletion (on the group) and attribute deletion (on the user)
	conn.ExpectModify(goldap.ModifyRequest{
		DN: "uid=alice,ou=users,dc=example,dc=org",
		Changes: []goldap.Change{{
			Operation:    goldap.DeleteAttribute,
			Modification: goldap.PartialAttribute{Type: "isMemberOf"},
		}},
	})
	conn.ExpectDelete(goldap.DelRequest{
		DN: "cn=grafana-admins,ou=groups,dc=example,dc=org",
	})
	test.ExpectNoErrors(t, updateDBWithRunningAdapter(action))
	conn.CheckAllExecuted(t)
}

const dummySSHPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGNvYUluYODNXoQKDGG+pTEigpsvJP2SHfMz0a+Hl2xO alice@example.org"

// The password belonging to this hash is "foo".
const dummyPasswordHash = "$6$sxI7hpdrkEHuquNj$6zcRp52hrMXSFeF1EOrdETuVYmAmOsYCiG7sCCP54CoX8vHwCEUURWxY5Si0LyvRoC/oZPDaNjUh4DDFBO/Wi/"

func TestAllFieldsFilled(t *testing.T) {
	//This test puts values into all user/group fields, including the optional
	//ones, to check how those are rendered in the directory.
	conn, updateDBWithRunningAdapter := setupAdapterTest(t)

	//put one user and one group in the database
	action := func(db *core.Database) errext.ErrorSet {
		db.Users = []core.User{{
			LoginName:     "alice",
			GivenName:     "Alice",
			FamilyName:    "Administrator",
			EMailAddress:  "alice@example.org",
			SSHPublicKeys: []string{dummySSHPublicKey},
			PasswordHash:  dummyPasswordHash,
			POSIX: &core.UserPosixAttributes{
				UID:           1234,
				GID:           123,
				HomeDirectory: "/home/alice",
				LoginShell:    "/bin/zsh",
				GECOS:         "Alice Allison",
			},
		}}
		gid := core.PosixID(123)
		db.Groups = []core.Group{{
			Name:             "admins",
			LongName:         "Administrators",
			MemberLoginNames: core.GroupMemberNames{"alice": true},
			Permissions: core.Permissions{
				Portunus: core.PortunusPermissions{IsAdmin: true},
				LDAP:     core.LDAPPermissions{CanRead: true},
			},
			PosixGID: &gid,
		}}
		return nil
	}

	//check their rendering (because "all fields" includes
	//Group.Permissions.LDAP.CanRead, our test user ends up in the
	//"portunus-viewers" group)
	conn.ExpectAdd(goldap.AddRequest{
		DN: "uid=alice,ou=users,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "uid", Vals: []string{"alice"}},
			{Type: "cn", Vals: []string{"Alice Administrator"}},
			{Type: "sn", Vals: []string{"Administrator"}},
			{Type: "givenName", Vals: []string{"Alice"}},
			{Type: "userPassword", Vals: []string{dummyPasswordHash}},
			{Type: "mail", Vals: []string{"alice@example.org"}},
			{Type: "sshPublicKey", Vals: []string{dummySSHPublicKey}},
			{Type: "uidNumber", Vals: []string{"1234"}},
			{Type: "gidNumber", Vals: []string{"123"}},
			{Type: "homeDirectory", Vals: []string{"/home/alice"}},
			{Type: "loginShell", Vals: []string{"/bin/zsh"}},
			{Type: "gecos", Vals: []string{"Alice Allison"}},
			{Type: "isMemberOf", Vals: []string{"cn=admins,ou=groups,dc=example,dc=org"}},
			{Type: "objectClass", Vals: []string{"portunusPerson", "inetOrgPerson", "organizationalPerson", "person", "top", "posixAccount"}},
		},
	})
	conn.ExpectAdd(goldap.AddRequest{
		DN: "cn=admins,ou=groups,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "cn", Vals: []string{"admins"}},
			{Type: "member", Vals: []string{"uid=alice,ou=users,dc=example,dc=org"}},
			{Type: "objectClass", Vals: []string{"groupOfNames", "top"}},
		},
	})
	conn.ExpectAdd(goldap.AddRequest{
		DN: "cn=admins,ou=posix-groups,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "cn", Vals: []string{"admins"}},
			{Type: "gidNumber", Vals: []string{"123"}},
			{Type: "memberUid", Vals: []string{"alice"}},
			{Type: "objectClass", Vals: []string{"posixGroup", "top"}},
		},
	})
	conn.ExpectAdd(goldap.AddRequest{
		DN: "cn=portunus-viewers,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "cn", Vals: []string{"portunus-viewers"}},
			{Type: "member", Vals: []string{"uid=alice,ou=users,dc=example,dc=org"}},
			{Type: "objectClass", Vals: []string{"groupOfNames", "top"}},
		},
	})
	test.ExpectNoErrors(t, updateDBWithRunningAdapter(action))
	conn.CheckAllExecuted(t)
}

func TestTypeChanges(t *testing.T) {
	//This test checks how changes of regular users/groups into POSIX
	//users/groups and vice versa are reflected in the directory.
	conn, updateDBWithRunningAdapter := setupAdapterTest(t)

	//first we set up a regular user and group...
	action := func(db *core.Database) errext.ErrorSet {
		db.Users = []core.User{{
			LoginName:    "alice",
			GivenName:    "Alice",
			FamilyName:   "Administrator",
			PasswordHash: "x",
		}}
		db.Groups = []core.Group{{
			Name:             "admins",
			LongName:         "Administrators",
			MemberLoginNames: core.GroupMemberNames{"alice": true},
		}}
		return nil
	}

	//...so these objects should only be created with the default sets of object
	//classes, and the group only gets created in ou=groups (this first DB update
	//also creates the "portunus-viewers" group)
	conn.ExpectAdd(goldap.AddRequest{
		DN: "uid=alice,ou=users,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "uid", Vals: []string{"alice"}},
			{Type: "cn", Vals: []string{"Alice Administrator"}},
			{Type: "sn", Vals: []string{"Administrator"}},
			{Type: "givenName", Vals: []string{"Alice"}},
			{Type: "userPassword", Vals: []string{"x"}},
			{Type: "isMemberOf", Vals: []string{"cn=admins,ou=groups,dc=example,dc=org"}},
			{Type: "objectClass", Vals: []string{"portunusPerson", "inetOrgPerson", "organizationalPerson", "person", "top"}},
		},
	})
	conn.ExpectAdd(goldap.AddRequest{
		DN: "cn=admins,ou=groups,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "cn", Vals: []string{"admins"}},
			{Type: "member", Vals: []string{"uid=alice,ou=users,dc=example,dc=org"}},
			{Type: "objectClass", Vals: []string{"groupOfNames", "top"}},
		},
	})
	conn.ExpectAdd(goldap.AddRequest{
		DN: "cn=portunus-viewers,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "cn", Vals: []string{"portunus-viewers"}},
			{Type: "member", Vals: []string{"cn=nobody,dc=example,dc=org"}}, //placeholder because attribute is required
			{Type: "objectClass", Vals: []string{"groupOfNames", "top"}},
		},
	})
	test.ExpectNoErrors(t, updateDBWithRunningAdapter(action))
	conn.CheckAllExecuted(t)

	//changing the regular group into a POSIX group...
	action = func(db *core.Database) errext.ErrorSet {
		gid := core.PosixID(100)
		db.Groups[0].PosixGID = &gid
		return nil
	}

	//...creates an alternate representation of this group in ou=posix-groups
	conn.ExpectAdd(goldap.AddRequest{
		DN: "cn=admins,ou=posix-groups,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "cn", Vals: []string{"admins"}},
			{Type: "gidNumber", Vals: []string{"100"}},
			{Type: "memberUid", Vals: []string{"alice"}},
			{Type: "objectClass", Vals: []string{"posixGroup", "top"}},
		},
	})
	test.ExpectNoErrors(t, updateDBWithRunningAdapter(action))
	conn.CheckAllExecuted(t)

	//changing the regular user into a POSIX user...
	action = func(db *core.Database) errext.ErrorSet {
		db.Users[0].POSIX = &core.UserPosixAttributes{
			UID:           1000,
			GID:           100,
			HomeDirectory: "/home/alice",
		}
		return nil
	}

	//...adds the objectClass "posixAccount" and the respective attributes
	conn.ExpectModify(goldap.ModifyRequest{
		DN: "uid=alice,ou=users,dc=example,dc=org",
		Changes: []goldap.Change{
			{
				Operation: goldap.ReplaceAttribute,
				Modification: goldap.PartialAttribute{
					Type: "objectClass",
					Vals: []string{"portunusPerson", "inetOrgPerson", "organizationalPerson", "person", "top", "posixAccount"}},
			},
			{
				Operation:    goldap.ReplaceAttribute,
				Modification: goldap.PartialAttribute{Type: "uidNumber", Vals: []string{"1000"}},
			},
			{
				Operation:    goldap.ReplaceAttribute,
				Modification: goldap.PartialAttribute{Type: "gidNumber", Vals: []string{"100"}},
			},
			{
				Operation:    goldap.ReplaceAttribute,
				Modification: goldap.PartialAttribute{Type: "gecos", Vals: []string{"Alice Administrator"}},
			},
			{
				Operation:    goldap.ReplaceAttribute,
				Modification: goldap.PartialAttribute{Type: "homeDirectory", Vals: []string{"/home/alice"}},
			},
		},
	})
	test.ExpectNoErrors(t, updateDBWithRunningAdapter(action))
	conn.CheckAllExecuted(t)

	//we are going to change the group back first in order to cover every pairing
	//of user type and group type -- changing the POSIX group back into a regular
	//group...
	action = func(db *core.Database) errext.ErrorSet {
		db.Groups[0].PosixGID = nil
		return nil
	}

	//...removes the alternate representation in ou=posix-groups, keeping only the default representation in ou=groups
	conn.ExpectDelete(goldap.DelRequest{
		DN: "cn=admins,ou=posix-groups,dc=example,dc=org",
	})
	test.ExpectNoErrors(t, updateDBWithRunningAdapter(action))
	conn.CheckAllExecuted(t)

	//changing the POSIX user back into a regular user...
	action = func(db *core.Database) errext.ErrorSet {
		db.Users[0].POSIX = nil
		return nil
	}

	//...removes that objectClass and its attributes again
	conn.ExpectModify(goldap.ModifyRequest{
		DN: "uid=alice,ou=users,dc=example,dc=org",
		Changes: []goldap.Change{
			{
				Operation: goldap.ReplaceAttribute,
				Modification: goldap.PartialAttribute{
					Type: "objectClass",
					Vals: []string{"portunusPerson", "inetOrgPerson", "organizationalPerson", "person", "top"}},
			},
			{Operation: goldap.DeleteAttribute, Modification: goldap.PartialAttribute{Type: "uidNumber"}},
			{Operation: goldap.DeleteAttribute, Modification: goldap.PartialAttribute{Type: "gidNumber"}},
			{Operation: goldap.DeleteAttribute, Modification: goldap.PartialAttribute{Type: "gecos"}},
			{Operation: goldap.DeleteAttribute, Modification: goldap.PartialAttribute{Type: "homeDirectory"}},
		},
	})
	test.ExpectNoErrors(t, updateDBWithRunningAdapter(action))
	conn.CheckAllExecuted(t)
}

func TestLDAPViewerPermission(t *testing.T) {
	//This test checks that flipping Permissions.LDAP.CanRead on an existing
	//group populates the virtual group correctly.
	conn, updateDBWithRunningAdapter := setupAdapterTest(t)

	//first we set up a user and group without LDAP permissions...
	action := func(db *core.Database) errext.ErrorSet {
		db.Users = []core.User{{
			LoginName:    "alice",
			GivenName:    "Alice",
			FamilyName:   "Administrator",
			PasswordHash: "x",
		}}
		db.Groups = []core.Group{{
			Name:             "admins",
			LongName:         "Administrators",
			MemberLoginNames: core.GroupMemberNames{"alice": true},
		}}
		return nil
	}

	//...so the "portunus-viewers" group will be empty
	conn.ExpectAdd(goldap.AddRequest{
		DN: "uid=alice,ou=users,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "uid", Vals: []string{"alice"}},
			{Type: "cn", Vals: []string{"Alice Administrator"}},
			{Type: "sn", Vals: []string{"Administrator"}},
			{Type: "givenName", Vals: []string{"Alice"}},
			{Type: "userPassword", Vals: []string{"x"}},
			{Type: "isMemberOf", Vals: []string{"cn=admins,ou=groups,dc=example,dc=org"}},
			{Type: "objectClass", Vals: []string{"portunusPerson", "inetOrgPerson", "organizationalPerson", "person", "top"}},
		},
	})
	conn.ExpectAdd(goldap.AddRequest{
		DN: "cn=admins,ou=groups,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "cn", Vals: []string{"admins"}},
			{Type: "member", Vals: []string{"uid=alice,ou=users,dc=example,dc=org"}},
			{Type: "objectClass", Vals: []string{"groupOfNames", "top"}},
		},
	})
	conn.ExpectAdd(goldap.AddRequest{
		DN: "cn=portunus-viewers,dc=example,dc=org",
		Attributes: []goldap.Attribute{
			{Type: "cn", Vals: []string{"portunus-viewers"}},
			{Type: "member", Vals: []string{"cn=nobody,dc=example,dc=org"}}, //placeholder because attribute is required
			{Type: "objectClass", Vals: []string{"groupOfNames", "top"}},
		},
	})
	test.ExpectNoErrors(t, updateDBWithRunningAdapter(action))
	conn.CheckAllExecuted(t)

	//adding the LDAP permission on the group...
	action = func(db *core.Database) errext.ErrorSet {
		db.Groups[0].Permissions.LDAP.CanRead = true
		return nil
	}

	//...should add the user in that group to the "portunus-viewers" group
	//(since "portunus-viewers" is a virtual group, it does not appear in the
	//"isMemberOf" attribute of the user)
	conn.ExpectModify(goldap.ModifyRequest{
		DN: "cn=portunus-viewers,dc=example,dc=org",
		Changes: []goldap.Change{{
			Operation:    goldap.ReplaceAttribute,
			Modification: goldap.PartialAttribute{Type: "member", Vals: []string{"uid=alice,ou=users,dc=example,dc=org"}},
		}},
	})
	test.ExpectNoErrors(t, updateDBWithRunningAdapter(action))
	conn.CheckAllExecuted(t)
}
