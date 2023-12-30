/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import (
	"context"
	"sort"
	"testing"

	"github.com/majewsky/portunus/internal/crypt"
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
				PasswordHash:  "{PLAINTEXT}swordfish",
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

func reducerReturnEmpty(db *Database) errext.ErrorSet {
	*db = Database{}
	return nil
}

// Reducer: Given the DB from dbWithBasicSeedApplied(), overwrite all seeded
// attributes on "maxuser" and "maxgroup". (They are both at index 0 because
// the normalization sorts by identifier.)
func reducerOverwriteSeededAttrs1(hasher crypt.PasswordHasher) func(*Database) errext.ErrorSet {
	return func(db *Database) errext.ErrorSet {
		db.Groups[0].LongName += "-changed"
		db.Groups[0].MemberLoginNames = GroupMemberNames{} //removing seeded members is not allowed
		db.Groups[0].Permissions.Portunus.IsAdmin = true
		db.Groups[0].Permissions.LDAP.CanRead = false
		db.Groups[0].PosixGID = pointerTo(*db.Groups[0].PosixGID + 1)
		db.Users[0].GivenName += "-changed"
		db.Users[0].FamilyName += "-changed"
		db.Users[0].EMailAddress = "changed@example.org"
		db.Users[0].SSHPublicKeys = append(db.Users[0].SSHPublicKeys, dummySSHPublicKey)
		db.Users[0].PasswordHash = hasher.HashPassword("incorrect")
		db.Users[0].POSIX.UID += 1
		db.Users[0].POSIX.GID += 1
		db.Users[0].POSIX.HomeDirectory += "-changed"
		db.Users[0].POSIX.LoginShell += "-changed"
		db.Users[0].POSIX.GECOS += "-changed"
		return nil
	}
}

// Reducer: Like reducerOverwriteSeededAttrs1, but this one contains some edits
// that conflicts with the edits in that reducer.
func reducerOverwriteSeededAttrs2(db *Database) errext.ErrorSet {
	db.Users[0].POSIX = nil
	return nil
}

// Reducer: Given the DB from dbWithBasicSeedApplied(), rename "maxuser" and
// "maxgroup". (They are both at index 0 because the normalization sorts by
// identifier.)
func reducerOverwriteSeededIdentifiers(db *Database) errext.ErrorSet {
	previousUserName := db.Users[0].LoginName
	db.Groups[0].Name += "-renamed"
	db.Users[0].LoginName += "-renamed"

	//avoid complaints about an invalid group membership (that's not what we're testing here)
	db.Groups[0].MemberLoginNames[previousUserName] = false

	return nil
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
func reducerOverwriteMalleableAttributes(hasher crypt.PasswordHasher) func(*Database) errext.ErrorSet {
	return func(db *Database) errext.ErrorSet {
		db.Users[0].PasswordHash = hasher.HashPassword("swordfish")
		db.Groups[0].MemberLoginNames["minuser"] = true
		return nil
	}
}

// Reducer: Given the DB from dbWithBasicSeedApplied(), overwrite all unseeded
// attributes on "minuser" and "mingroup". (They are both at index 1 because
// the normalization sorts by identifier.)
func reducerOverwriteUnseededAttributes(hasher crypt.PasswordHasher) func(*Database) errext.ErrorSet {
	return func(db *Database) errext.ErrorSet {
		db.Groups[1].MemberLoginNames = GroupMemberNames{"minuser": true} //removing seeded members is not allowed
		db.Groups[1].Permissions.Portunus.IsAdmin = true
		db.Groups[1].Permissions.LDAP.CanRead = true
		db.Groups[1].PosixGID = pointerTo(PosixID(123))
		db.Users[1].EMailAddress = "minuser@example.org"
		db.Users[1].SSHPublicKeys = []string{dummySSHPublicKey}
		db.Users[1].PasswordHash = hasher.HashPassword("qwerty")
		db.Users[1].POSIX = &UserPosixAttributes{
			UID:           142,
			GID:           123,
			HomeDirectory: "/home/minuser",
			LoginShell:    "/bin/sh",
			GECOS:         "Minimal User",
		}
		return nil
	}
}

func TestSeedEnforcementRelaxed(t *testing.T) {
	//This test initializes an empty database using `fixtures/seed-basic.json`
	//and then executes various reducers on it. This test is "relaxed" in the
	//sense that updates do not use ConflictWithSeedIsError, so conflicts will be
	//corrected silently and most reducers turn into no-ops.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	vcfg := GetValidationConfigForTests()
	seed, errs := ReadDatabaseSeed("fixtures/seed-basic.json", vcfg)
	expectNoErrors(t, errs)

	//register a listener to observe the real DB changes
	hasher := &NoopHasher{}
	nexus := NewNexus(seed, vcfg, hasher)
	var actualDB Database
	nexus.AddListener(ctx, func(db Database) {
		actualDB = db
	})

	//load an empty database (like on first startup) -> seed gets applied
	errs = nexus.Update(reducerReturnEmpty, nil)
	expectNoErrors(t, errs)

	expectedDB := dbWithBasicSeedApplied()
	assert.DeepEqual(t, "database contents", actualDB, expectedDB)

	//overwriting seeded attributes is not allowed
	//-> no change because seed gets reenforced
	errs = nexus.Update(reducerOverwriteSeededAttrs1(hasher), nil)
	expectNoErrors(t, errs)
	assert.DeepEqual(t, "database contents", actualDB, expectedDB)

	errs = nexus.Update(reducerOverwriteSeededAttrs2, nil)
	expectNoErrors(t, errs)
	assert.DeepEqual(t, "database contents", actualDB, expectedDB)

	//overwriting seeded attributes in a compatible way is allowed
	errs = nexus.Update(reducerOverwriteMalleableAttributes(hasher), nil)
	expectNoErrors(t, errs)

	err := reducerOverwriteMalleableAttributes(hasher)(&expectedDB)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, "database contents", actualDB, expectedDB)

	//overwriting unseeded attributes is always allowed
	errs = nexus.Update(reducerOverwriteUnseededAttributes(hasher), nil)
	expectNoErrors(t, errs)

	err = reducerOverwriteUnseededAttributes(hasher)(&expectedDB)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, "database contents", actualDB, expectedDB)
}

func TestSeedEnforcementStrict(t *testing.T) {
	//Same as TestSeedEnforcementRelaxed, but this test is "strict" in the sense
	//that all updates set ConflictWithSeedIsError. Therefore, most of them fail
	//instead of turning into silent no-ops.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	vcfg := GetValidationConfigForTests()
	seed, errs := ReadDatabaseSeed("fixtures/seed-basic.json", vcfg)
	expectNoErrors(t, errs)

	//register a listener to observe the real DB changes
	hasher := &NoopHasher{}
	nexus := NewNexus(seed, vcfg, hasher)
	var actualDB Database
	updateCount := 0
	nexus.AddListener(ctx, func(db Database) {
		actualDB = db
		updateCount++
	})

	//load an empty database (like on first startup) -> seed gets applied
	errs = nexus.Update(reducerReturnEmpty, nil)
	expectNoErrors(t, errs)

	expectedDB := dbWithBasicSeedApplied()
	assert.DeepEqual(t, "database contents", actualDB, expectedDB)
	assert.DeepEqual(t, "update count", updateCount, 1)

	//overwriting seeded attributes is not allowed
	opts := UpdateOptions{ConflictWithSeedIsError: true}
	errs = nexus.Update(reducerOverwriteSeededAttrs1(hasher), &opts)
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
	errs = nexus.Update(reducerOverwriteMalleableAttributes(hasher), &opts)
	expectNoErrors(t, errs)

	err := reducerOverwriteMalleableAttributes(hasher)(&expectedDB)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, "database contents", actualDB, expectedDB)
	assert.DeepEqual(t, "update count", updateCount, 2)

	//overwriting unseeded attributes is always allowed
	errs = nexus.Update(reducerOverwriteUnseededAttributes(hasher), &opts)
	expectNoErrors(t, errs)

	err = reducerOverwriteUnseededAttributes(hasher)(&expectedDB)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, "database contents", actualDB, expectedDB)
	assert.DeepEqual(t, "update count", updateCount, 3)
}

func TestSeedParseAndValidationErrors(t *testing.T) {
	vcfg := GetValidationConfigForTests()

	//test a seed file with unknown attributes (we parse with strict rules)
	_, errs := ReadDatabaseSeed("fixtures/seed-parse-error-1.json", vcfg)
	expectTheseErrors(t, errs,
		`while parsing fixtures/seed-parse-error-1.json: json: unknown field "unknown_attribute"`,
	)

	//test a seed file with a malformatted command substitution
	_, errs = ReadDatabaseSeed("fixtures/seed-parse-error-2.json", vcfg)
	expectTheseErrors(t, errs,
		`while parsing fixtures/seed-parse-error-2.json: json: cannot unmarshal object into Go struct field UserSeed.users.password of type string`,
	)

	//NOTE: We trust all other parse-level errors (e.g. GIDs/UIDs longer than 16
	//bit) to be caught by the JSON parser, since these constraints are all
	//expressed in the type system.

	//test a seed file with every possible validation error (each user and group
	//has one validation error, as indicated in their name fields)
	_, errs = ReadDatabaseSeed("fixtures/seed-validation-errors.json", vcfg)
	expectTheseErrors(t, errs,
		`field "login_name" in user "" is missing`,
		`field "login_name" in user " spaces-in-name " may not start with a space character`,
		`field "login_name" in user "malformed-name$" is not an acceptable user name`,
		`field "login_name" in user "nonposix.name" is not an acceptable POSIX account name matching the pattern /^[a-z_][a-z0-9_-]*\$?$/`,
		`field "login_name" in user "nonldap,name" may not include commas, plus signs or equals signs`,
		`field "login_name" in user "duplicate.name" is defined multiple times`,
		`field "given_name" in user "missing-given-name" is missing`,
		`field "given_name" in user "spaces-in-given-name" may not start with a space character`,
		`field "family_name" in user "missing-family-name" is missing`,
		`field "family_name" in user "spaces-in-family-name" may not end with a space character`,
		`field "ssh_public_keys" in user "only-ssh-key-empty" must have a valid SSH public key on each line (parse error on line 1)`,
		`field "ssh_public_keys" in user "some-ssh-key-empty" must have a valid SSH public key on each line (parse error on line 2)`,
		`field "ssh_public_keys" in user "ssh-key-invalid" must have a valid SSH public key on each line (parse error on line 1)`,
		`field "posix_uid" in user "posix-no-uid" is missing`,
		`field "posix_gid" in user "posix-no-gid" is missing`,
		`field "posix_home" in user "posix-no-home" is missing`,
		`field "posix_home" in user "posix-spaces-in-home" may not end with a space character`,
		`field "posix_home" in user "posix-home-is-not-absolute" must be an absolute path, i.e. start with a /`,
		`field "posix_shell" in user "posix-shell-is-not-absolute" must be an absolute path, i.e. start with a /`,
		`field "name" in group "" is missing`,
		`field "name" in group " spaces-in-name " may not start with a space character`,
		`field "name" in group "malformed-name$" is not an acceptable group name`,
		`field "name" in group "nonposix.name" is not an acceptable POSIX account name matching the pattern /^[a-z_][a-z0-9_-]*\$?$/`,
		`field "name" in group "nonldap,name" may not include commas, plus signs or equals signs`,
		`field "name" in group "duplicate.name" is defined multiple times`,
		`field "long_name" in group "missing-long-name" is missing`,
		`field "long_name" in group "spaces-in-long-name" may not start with a space character`,
		`field "members" in group "unknown-member" contains unknown user with login name "incognito"`,
	)
}

func TestSeedCryptoAgility(t *testing.T) {
	//This test initializes a database from seed with one minimal user that has a
	//seeded password. We then test how the seed application and verification
	//behaves when the seeded password is hashed with different methods.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	vcfg := GetValidationConfigForTests()
	seed, errs := ReadDatabaseSeed("fixtures/seed-one-user-with-password.json", vcfg)
	expectNoErrors(t, errs)
	_ = seed

	//register a listener to observe the real DB changes
	hasher := &NoopHasher{}
	nexus := NewNexus(seed, vcfg, hasher)
	var actualDB Database
	nexus.AddListener(ctx, func(db Database) {
		actualDB = db
	})

	//load an empty database (like on first startup) -> seed gets applied
	errs = nexus.Update(reducerReturnEmpty, nil)
	expectNoErrors(t, errs)

	expectedDB := Database{
		Users: []User{{
			LoginName:    "minuser",
			GivenName:    "Minimal",
			FamilyName:   "User",
			PasswordHash: "{PLAINTEXT}swordfish",
		}},
		Groups: []Group{},
	}
	assert.DeepEqual(t, "database contents", actualDB, expectedDB)

	//change to a different hash method, but the hash still matches the password
	//-> this will be accepted since this hash method is not considered weak
	errs = nexus.Update(func(db *Database) errext.ErrorSet {
		db.Users[0].PasswordHash = "{WEAK-PLAINTEXT}swordfish"
		return nil
	}, nil)
	expectNoErrors(t, errs)

	expectedDB.Users[0].PasswordHash = "{WEAK-PLAINTEXT}swordfish"
	assert.DeepEqual(t, "database contents", actualDB, expectedDB)

	//if the hash method is considered weak, DatabaseSeed.ApplyTo() will rehash
	//using a stronger method
	hasher.UpgradeWeakHashes = true
	errs = nexus.Update(func(db *Database) errext.ErrorSet { return nil }, nil)
	expectNoErrors(t, errs)

	expectedDB.Users[0].PasswordHash = "{PLAINTEXT}swordfish"
	assert.DeepEqual(t, "database contents", actualDB, expectedDB)
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
