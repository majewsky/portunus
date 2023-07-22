/*******************************************************************************
* Copyright 2019-2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package ldap

import (
	"context"
	"fmt"
	"strings"
	"sync"

	goldap "github.com/go-ldap/ldap/v3"
	"github.com/majewsky/portunus/internal/core"
)

// Adapter translates changes to the Portunus database into updates in the LDAP
// database.
type Adapter struct {
	nexus        core.Nexus
	conn         Connection
	init         sync.Once
	objects      []Object //persisted objects, key = object DN
	objectsMutex sync.Mutex
}

// NewAdapter initializes an Adapter instance.
func NewAdapter(nexus core.Nexus, conn Connection) *Adapter {
	return &Adapter{nexus: nexus, conn: conn}
}

// Run listens for changes to the Portunus database until `ctx` expires.
// An error is returned if any write into the LDAP database fails.
func (a *Adapter) Run(ctx context.Context) error {
	operationsChan := make(chan operation, 64)

	//create main directory structure, but only when Run() is called for the first time
	//(this precaution is not relevant for regular execution because main() calls
	//Run() exactly once anyway; however, unit tests call Run() multiple times to
	//test LDAP activity with fine granularity)
	isFirstRun := false
	a.init.Do(func() { isFirstRun = true })
	if isFirstRun {
		for _, addReq := range makeStaticObjects(a.conn.DNSuffix()) {
			err := a.conn.Add(addReq)
			if err != nil {
				return err
			}
		}
	}

	//when the nexus informs us about a DB change, we compute the set of LDAP
	//operations immediately; but then we send them back to this goroutine for
	//execution to ensure proper serialization and to allow the nexus to proceed
	//with the update before the LDAP server has finished ingesting our
	//operations
	a.nexus.AddListener(ctx, func(db core.Database) {
		newObjects := renderDBToLDAP(db, a.conn.DNSuffix())

		a.objectsMutex.Lock()
		defer a.objectsMutex.Unlock()

		computeUpdates(a.objects, newObjects, operationsChan)
		a.objects = newObjects
	})

	for {
		select {
		case <-ctx.Done():
			return nil
		case op := <-operationsChan:
			err := op.ExecuteOn(a.conn)
			if err != nil {
				return err
			}
		}
	}
}

// Renders the static objects that we need to establish our basic LDAP
// directory structure.
func makeStaticObjects(dnSuffix string) (result []goldap.AddRequest) {
	//shorthand for obtaining a goldap.Attribute object
	attr := func(typeName string, values ...string) goldap.Attribute {
		return goldap.Attribute{Type: typeName, Vals: values}
	}

	//domain-component object
	suffixRDNs := strings.Split(dnSuffix, ",")
	dcName := strings.TrimPrefix(suffixRDNs[0], "dc=")
	result = append(result, goldap.AddRequest{
		DN: dnSuffix,
		Attributes: []goldap.Attribute{
			attr("dc", dcName),
			attr("o", dcName),
			attr("objectClass", "dcObject", "organization", "top"),
		},
	})

	//organizational units
	for _, ouName := range []string{"users", "groups", "posix-groups"} {
		result = append(result, goldap.AddRequest{
			DN: fmt.Sprintf("ou=%s,%s", ouName, dnSuffix),
			Attributes: []goldap.Attribute{
				attr("ou", ouName),
				attr("objectClass", "organizationalUnit", "top"),
			},
		})
	}

	//service user account
	result = append(result, goldap.AddRequest{
		DN: "cn=portunus," + dnSuffix,
		Attributes: []goldap.Attribute{
			attr("cn", "portunus"),
			attr("description", "Internal service user for Portunus"),
			attr("objectClass", "organizationalRole", "top"),
		},
	})

	//dummy user account for empty groups
	result = append(result, goldap.AddRequest{
		DN: "cn=nobody," + dnSuffix,
		Attributes: []goldap.Attribute{
			attr("cn", "nobody"),
			attr("description", "Dummy user for empty groups (all groups need to have at least one member)"),
			attr("objectClass", "organizationalRole", "top"),
		},
	})

	return result
}

// Converts a core.Database instance into a list of LDAP objects.
func renderDBToLDAP(db core.Database, dnSuffix string) (result []Object) {
	for _, u := range db.Users {
		result = append(result, renderUser(u, dnSuffix, db.Groups))
	}
	for _, g := range db.Groups {
		result = append(result, renderGroup(g, dnSuffix)...)
	}

	//render the virtual group that controls read access to the LDAP server (this
	//group is hardcoded in the LDAP server's ACL)
	var ldapViewerDNames []string
	for _, group := range db.Groups {
		if group.Permissions.LDAP.CanRead {
			for loginName, isMember := range group.MemberLoginNames {
				if isMember {
					dn := fmt.Sprintf("uid=%s,ou=users,%s", loginName, dnSuffix)
					ldapViewerDNames = append(ldapViewerDNames, dn)
				}
			}
		}
	}
	if len(ldapViewerDNames) == 0 {
		//groups need to have at least one member
		ldapViewerDNames = append(ldapViewerDNames, "cn=nobody,"+dnSuffix)
	}
	result = append(result, Object{
		DN: fmt.Sprintf("cn=portunus-viewers,%s", dnSuffix),
		Attributes: map[string][]string{
			"cn":          {"portunus-viewers"},
			"member":      ldapViewerDNames,
			"objectClass": {"groupOfNames", "top"},
		},
	})

	return
}
