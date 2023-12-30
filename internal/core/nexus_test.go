/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import (
	"context"
	"testing"

	"github.com/sapcc/go-bits/assert"
	"github.com/sapcc/go-bits/errext"
)

// NOTE: Most actual test coverage for the Nexus (esp. the validation logic) is
// in the seed tests. These tests cover some specific behaviors that are
// irrelevant to the seeding, but important for UI workflows.

func TestDryRun(t *testing.T) {
	//This test checks the behavior of the `UpdateOptions.DryRun` flag.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	vcfg := GetValidationConfigForTests()
	hasher := &NoopHasher{}
	nexus := NewNexus(nil, vcfg, hasher)
	var actualDB Database
	nexus.AddListener(ctx, func(db Database) {
		actualDB = db
	})

	//load a minimal database
	actionLoadEmpty := func(db *Database) errext.ErrorSet {
		db.Users = []User{{
			LoginName:  "minuser",
			GivenName:  "Minimal",
			FamilyName: "User",
		}}
		db.Groups = []Group{{
			Name:             "mingroup",
			LongName:         "Minimal Group",
			MemberLoginNames: GroupMemberNames{},
		}}
		return nil
	}
	errs := nexus.Update(actionLoadEmpty, nil)
	expectNoErrors(t, errs)
	assert.DeepEqual(t, "user given name", actualDB.Users[0].FullName(), "Minimal User")

	//an action that fails...
	actionFail := func(db *Database) (errs errext.ErrorSet) {
		db.Users[0].GivenName = "Changed"
		errs.Addf("error from action")
		return
	}

	//...will behave the same with DryRun and without
	errs = nexus.Update(actionFail, &UpdateOptions{DryRun: true})
	expectTheseErrors(t, errs, "error from action")
	assert.DeepEqual(t, "user given name", actualDB.Users[0].FullName(), "Minimal User")

	errs = nexus.Update(actionFail, nil)
	expectTheseErrors(t, errs, "error from action")
	assert.DeepEqual(t, "user given name", actualDB.Users[0].FullName(), "Minimal User")

	//an action that succeeds...
	counter := 0
	actionSucceed := func(db *Database) (errs errext.ErrorSet) {
		db.Users[0].GivenName = "Changed"
		counter++
		return
	}

	//...will run, but not be committed under DryRun
	errs = nexus.Update(actionSucceed, &UpdateOptions{DryRun: true})
	expectNoErrors(t, errs)
	assert.DeepEqual(t, "user given name", actualDB.Users[0].FullName(), "Minimal User")
	assert.DeepEqual(t, "run counter", counter, 1)

	errs = nexus.Update(actionSucceed, nil)
	expectNoErrors(t, errs)
	assert.DeepEqual(t, "user given name", actualDB.Users[0].FullName(), "Changed User")
	assert.DeepEqual(t, "run counter", counter, 2)
}
