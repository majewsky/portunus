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
)

func setupAdapterTest(t *testing.T) (nexus core.Nexus, conn *test.LDAPConnectionDouble, executeWithRunningAdapter func(func())) {
	nexus = core.NewNexus()
	conn = test.NewLDAPConnectionDouble("dc=example,dc=org")
	adapter := NewAdapter(nexus, conn)

	//This can be used by the test to execute an action while adapter.Run() is
	//running in a separate goroutine. This function takes care to shutdown
	//adapter.Run() before it returns, thus ensuring that all effects of the
	//action on the LDAPConnectionDouble have been observed.
	executeWithRunningAdapter = func(action func()) {
		//This log is a breadcrumb for associating a failed adapter.Run() with the
		//respective executeWithRunningAdapter() callsite. We cannot use the
		//default callsite attribution through a goroutine boundary.
		t.Helper()
		t.Log("executeWithRunningAdapter running")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			test.ExpectNoError(t, adapter.Run(ctx))
		}()

		action()
		time.Sleep(10 * time.Millisecond) //give the Adapter some time to complete outstanding actions
		cancel()
		wg.Wait()
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

func TestMinimal(t *testing.T) {
	_, conn, executeWithRunningAdapter := setupAdapterTest(t)

	executeWithRunningAdapter(func() {
		//no action here: the Adapter will listen on the Nexus, but since the Nexus
		//has not received a DB from anywhere, it will not call into the Adapter
	})
	conn.CheckAllExecuted(t)
}

func TestBasicOperations(t *testing.T) {
	//The focus of this test is just to see that basic add/modify/delete
	//operations work. If possible, please add a new test instead of extending
	//this one.
	nexus, conn, executeWithRunningAdapter := setupAdapterTest(t)

	//when we add a user and group...
	action := func(db core.Database) (core.Database, error) {
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
		return db, nil
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
	executeWithRunningAdapter(func() {
		test.ExpectNoErrors(t, nexus.Update(action, nil))
	})
	conn.CheckAllExecuted(t)

	//there is no rename operation: when we change the RDN of an object...
	action = func(db core.Database) (core.Database, error) {
		db.Groups[0].Name = "grafana-admins"
		return db, nil
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
	executeWithRunningAdapter(func() {
		test.ExpectNoErrors(t, nexus.Update(action, nil))
	})
	conn.CheckAllExecuted(t)

	//we can test object updates by adding the user to the group...
	action = func(db core.Database) (core.Database, error) {
		db.Groups[0].MemberLoginNames = core.GroupMemberNames{db.Users[0].LoginName: true}
		return db, nil
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
	executeWithRunningAdapter(func() {
		test.ExpectNoErrors(t, nexus.Update(action, nil))
	})
	conn.CheckAllExecuted(t)

	//by deleting the group...
	action = func(db core.Database) (core.Database, error) {
		db.Groups = nil
		return db, nil
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
	executeWithRunningAdapter(func() {
		test.ExpectNoErrors(t, nexus.Update(action, nil))
	})
	conn.CheckAllExecuted(t)
}

//TODO: test type change between regular and POSIX user
//TODO: test type change between regular and POSIX group
//TODO: test LDAP viewer permissions
//TODO: test rendering of all possible fields
