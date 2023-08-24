/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/majewsky/portunus/internal/shared"
	"github.com/sapcc/go-bits/assert"
	"github.com/sapcc/go-bits/errext"
)

const dummySSHPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGNvYUluYODNXoQKDGG+pTEigpsvJP2SHfMz0a+Hl2xO maxuser@example.org"

// Returns the Database that results when applying `fixtures/seed-basic.json`
// to an empty Database.
func dbWithBasicSeedApplied() Database {
	//For each object type, the basic seed contains one minimal object (with only
	//the minimum set of required attributes) and one maximal object (with the
	//full possible set of seedable attributes). The minimal object is used to
	//test how the unspecified fields are filled in on seeding, and to verify
	//that the unspecified fields can be overridden manually. The maximal object
	//is used to test that the specified fields cannot be overriden manually.
	return Database{
		Groups: []Group{
			{
				Name:             "maxgroup",
				LongName:         "Maximal Group",
				MemberLoginNames: GroupMemberNames{"maxuser": true},
				Permissions: Permissions{
					LDAP: LDAPPermissions{CanRead: true},
				},
				PosixGID: pointerTo(PosixID(23)),
			},
			{
				Name:             "mingroup",
				LongName:         "Minimal Group",
				MemberLoginNames: GroupMemberNames{},
			},
		},
		Users: []User{
			{
				LoginName:     "maxuser",
				GivenName:     "Maximal",
				FamilyName:    "User",
				EMailAddress:  "maxuser@example.org",
				SSHPublicKeys: []string{dummySSHPublicKey},
				PasswordHash:  "matches:swordfish", //NOTE: see normalizeDBForComparison()
				POSIX: &UserPosixAttributes{
					UID:           42,
					GID:           23,
					HomeDirectory: "/home/maxuser",
					LoginShell:    "/bin/bash",
					GECOS:         "Maximal User",
				},
			},
			{
				LoginName:  "minuser",
				GivenName:  "Minimal",
				FamilyName: "User",
			},
		},
	}
}

func reducerReturnEmpty(_ Database) (Database, error) {
	return Database{}, nil
}

// Reducer: Given the DB from dbWithBasicSeedApplied(), overwrite all seeded
// attributes on "maxuser" and "maxgroup". (They are both at index 0 because
// the normalization sorts by identifier.)
func reducerOverwriteSeededAttrs1(db Database) (Database, error) {
	db.Groups[0].LongName += "-changed"
	db.Groups[0].MemberLoginNames = GroupMemberNames{} //removing seeded members is not allowed
	db.Groups[0].Permissions.Portunus.IsAdmin = true
	db.Groups[0].Permissions.LDAP.CanRead = false
	db.Groups[0].PosixGID = pointerTo(*db.Groups[0].PosixGID + 1)
	db.Users[0].GivenName += "-changed"
	db.Users[0].FamilyName += "-changed"
	db.Users[0].EMailAddress = "changed@example.org"
	db.Users[0].SSHPublicKeys = append(db.Users[0].SSHPublicKeys, dummySSHPublicKey)
	db.Users[0].PasswordHash = shared.HashPasswordForLDAP("incorrect")
	db.Users[0].POSIX.UID += 1
	db.Users[0].POSIX.GID += 1
	db.Users[0].POSIX.HomeDirectory += "-changed"
	db.Users[0].POSIX.LoginShell += "-changed"
	db.Users[0].POSIX.GECOS += "-changed"
	return db, nil
}

// Reducer: Like reducerOverwriteSeededAttrs1, but this one contains some edits
// that conflicts with the edits in that reducer.
func reducerOverwriteSeededAttrs2(db Database) (Database, error) {
	db.Users[0].POSIX = nil
	return db, nil
}

// Reducer: Given the DB from dbWithBasicSeedApplied(), rename "maxuser" and
// "maxgroup". (They are both at index 0 because the normalization sorts by
// identifier.)
func reducerOverwriteSeededIdentifiers(db Database) (Database, error) {
	db.Groups[0].Name += "-renamed"
	db.Users[0].LoginName += "-renamed"
	return db, nil
}

// Reducer: Given the DB from dbWithBasicSeedApplied(), overwrite seeded that
// allow certain edits.
//
// - The password hash for "maxuser" is overwritten, but without changing the
// accepted password.
// - The member list for "maxgroup" is extended, but without removing existing
// members.
//
// These changes are permissible even though the respective attributes are
// seeded, since the change does not conflict with the seed.
func reducerOverwriteMalleableAttributes(db Database) (Database, error) {
	db.Users[0].PasswordHash = shared.HashPasswordForLDAP("swordfish")
	db.Groups[0].MemberLoginNames["minuser"] = true
	return db, nil
}

// Reducer: Given the DB from dbWithBasicSeedApplied(), overwrite all unseeded
// attributes on "minuser" and "mingroup". (They are both at index 1 because
// the normalization sorts by identifier.)
func reducerOverwriteUnseededAttributes(db Database) (Database, error) {
	db.Groups[1].MemberLoginNames = GroupMemberNames{"minuser": true} //removing seeded members is not allowed
	db.Groups[1].Permissions.Portunus.IsAdmin = true
	db.Groups[1].Permissions.LDAP.CanRead = true
	db.Groups[1].PosixGID = pointerTo(PosixID(123))
	db.Users[1].EMailAddress = "minuser@example.org"
	db.Users[1].SSHPublicKeys = []string{dummySSHPublicKey}
	db.Users[1].PasswordHash = shared.HashPasswordForLDAP("qwerty")
	db.Users[1].POSIX = &UserPosixAttributes{
		UID:           142,
		GID:           123,
		HomeDirectory: "/home/minuser",
		LoginShell:    "/bin/sh",
		GECOS:         "Minimal User",
	}
	return db, nil
}

func TestSeedEnforcementRelaxed(t *testing.T) {
	//This test initializes an empty database using `fixtures/seed-basic.json`
	//and then executes various reducers on it. This test is "relaxed" in the
	//sense that updates do not use ConflictWithSeedIsError, so conflicts will be
	//corrected silently and most reducers turn into no-ops.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seed, err := ReadDatabaseSeed("fixtures/seed-basic.json")
	if err != nil {
		t.Fatal(err)
	}

	//register a listener to observe the real DB changes
	nexus := NewNexus(seed)
	var actualDB Database
	nexus.AddListener(ctx, func(db Database) {
		actualDB = db
	})

	//load an empty database (like on first startup) -> seed gets applied
	errs := nexus.Update(reducerReturnEmpty, nil)
	expectNoErrors(t, errs)

	expectedDB := dbWithBasicSeedApplied()
	normalizeDBForComparison(&actualDB)
	assert.DeepEqual(t, "database contents", actualDB, expectedDB)

	//overwriting seeded attributes is not allowed
	//-> no change because seed gets reenforced
	errs = nexus.Update(reducerOverwriteSeededAttrs1, nil)
	expectNoErrors(t, errs)
	normalizeDBForComparison(&actualDB)
	assert.DeepEqual(t, "database contents", actualDB, expectedDB)

	errs = nexus.Update(reducerOverwriteSeededAttrs2, nil)
	expectNoErrors(t, errs)
	normalizeDBForComparison(&actualDB)
	assert.DeepEqual(t, "database contents", actualDB, expectedDB)

	//overwriting seeded attributes in a compatible way is allowed
	errs = nexus.Update(reducerOverwriteMalleableAttributes, nil)
	expectNoErrors(t, errs)

	expectedDB, err = reducerOverwriteMalleableAttributes(expectedDB)
	if err != nil {
		t.Fatal(err)
	}
	normalizeDBForComparison(&expectedDB)
	normalizeDBForComparison(&actualDB)
	assert.DeepEqual(t, "database contents", actualDB, expectedDB)

	//overwriting unseeded attributes is always allowed
	errs = nexus.Update(reducerOverwriteUnseededAttributes, nil)
	expectNoErrors(t, errs)

	expectedDB, err = reducerOverwriteUnseededAttributes(expectedDB)
	if err != nil {
		t.Fatal(err)
	}
	normalizeDBForComparison(&expectedDB)
	normalizeDBForComparison(&actualDB)
	assert.DeepEqual(t, "database contents", actualDB, expectedDB)
}

func TestSeedEnforcementStrict(t *testing.T) {
	//Same as TestSeedEnforcementRelaxed, but this test is "strict" in the sense
	//that all updates set ConflictWithSeedIsError. Therefore, most of them fail
	//instead of turning into silent no-ops.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seed, err := ReadDatabaseSeed("fixtures/seed-basic.json")
	if err != nil {
		t.Fatal(err)
	}

	//register a listener to observe the real DB changes
	nexus := NewNexus(seed)
	var actualDB Database
	updateCount := 0
	nexus.AddListener(ctx, func(db Database) {
		actualDB = db
		updateCount++
	})

	//load an empty database (like on first startup) -> seed gets applied
	errs := nexus.Update(reducerReturnEmpty, nil)
	expectNoErrors(t, errs)

	expectedDB := dbWithBasicSeedApplied()
	normalizeDBForComparison(&actualDB)
	assert.DeepEqual(t, "database contents", actualDB, expectedDB)
	assert.DeepEqual(t, "update count", updateCount, 1)

	//overwriting seeded attributes is not allowed
	opts := UpdateOptions{ConflictWithSeedIsError: true}
	errs = nexus.Update(reducerOverwriteSeededAttrs1, &opts)
	expectTheseErrors(t, errs,
		`field "long_name" in group "maxgroup" must be equal to the seeded value`,
		`field "members" in group "maxgroup" must contain user "maxuser" because of seeded group membership`,
		`field "portunus_perms" in group "maxgroup" must be equal to the seeded value`,
		`field "ldap_perms" in group "maxgroup" must be equal to the seeded value`,
		`field "posix_gid" in group "maxgroup" must be equal to the seeded value`,
		`field "given_name" in user "maxuser" must be equal to the seeded value`,
		`field "family_name" in user "maxuser" must be equal to the seeded value`,
		`field "email" in user "maxuser" must be equal to the seeded value`,
		`field "ssh_public_keys" in user "maxuser" must be equal to the seeded value`,
		`field "password" in user "maxuser" must be equal to the seeded value`,
		`field "posix_uid" in user "maxuser" must be equal to the seeded value`,
		`field "posix_gid" in user "maxuser" must be equal to the seeded value`,
		`field "posix_home" in user "maxuser" must be equal to the seeded value`,
		`field "posix_shell" in user "maxuser" must be equal to the seeded value`,
		`field "posix_gecos" in user "maxuser" must be equal to the seeded value`,
	)
	assert.DeepEqual(t, "update count", updateCount, 1) //same as before (listener was not called)

	errs = nexus.Update(reducerOverwriteSeededAttrs2, &opts)
	expectTheseErrors(t, errs,
		`field "posix" in user "maxuser" must be equal to the seeded value`,
	)
	assert.DeepEqual(t, "update count", updateCount, 1) //same as before (listener was not called)

	//renaming seeded objects is not allowed
	errs = nexus.Update(reducerOverwriteSeededIdentifiers, &opts)
	expectTheseErrors(t, errs,
		`group "maxgroup" is seeded and cannot be deleted`,
		`user "maxuser" is seeded and cannot be deleted`,
	)
	assert.DeepEqual(t, "update count", updateCount, 1) //same as before (listener was not called)

	//overwriting seeded attributes in a compatible way is allowed
	errs = nexus.Update(reducerOverwriteMalleableAttributes, &opts)
	expectNoErrors(t, errs)

	expectedDB, err = reducerOverwriteMalleableAttributes(expectedDB)
	if err != nil {
		t.Fatal(err)
	}
	normalizeDBForComparison(&expectedDB)
	normalizeDBForComparison(&actualDB)
	assert.DeepEqual(t, "database contents", actualDB, expectedDB)
	assert.DeepEqual(t, "update count", updateCount, 2)

	//overwriting unseeded attributes is always allowed
	errs = nexus.Update(reducerOverwriteUnseededAttributes, &opts)
	expectNoErrors(t, errs)

	expectedDB, err = reducerOverwriteUnseededAttributes(expectedDB)
	if err != nil {
		t.Fatal(err)
	}
	normalizeDBForComparison(&expectedDB)
	normalizeDBForComparison(&actualDB)
	assert.DeepEqual(t, "database contents", actualDB, expectedDB)
	assert.DeepEqual(t, "update count", updateCount, 3)
}

//TODO: test invalid seed files

func normalizeDBForComparison(db *Database) {
	//We want to compare `db` using reflect.DeepEqual(), but the User.PasswordHash field cannot be trivially compared. This function prepares `db` for DeepEqual comparison by replacing all User.PasswordHash values with their original passwords.
	dummyPasswords := []string{"swordfish", "qwerty", "incorrect"}
	for idx, user := range db.Users {
		if strings.HasPrefix(user.PasswordHash, "matches:") {
			continue //already normalized
		}
		for _, pw := range dummyPasswords {
			if CheckPasswordHash(pw, user.PasswordHash) {
				db.Users[idx].PasswordHash = "matches:" + pw
				break
			}
		}
	}
}

func expectNoErrors(t *testing.T, errs errext.ErrorSet) {
	t.Helper()
	for _, err := range errs {
		t.Errorf("expected no errors, but got: %s", err.Error())
	}
}

func expectTheseErrors(t *testing.T, errs errext.ErrorSet, expected ...string) {
	t.Helper()
	actual := make([]string, len(errs))
	for idx, err := range errs {
		actual[idx] = err.Error()
	}
	sort.Strings(actual)
	sort.Strings(expected)
	assert.DeepEqual(t, "error messages", actual, expected)
}

func pointerTo[T any](val T) *T {
	return &val
}
