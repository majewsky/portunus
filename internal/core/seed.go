/*******************************************************************************
* Copyright 2022 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"

	"github.com/majewsky/portunus/internal/crypt"
	"github.com/sapcc/go-bits/errext"
	"github.com/sapcc/go-bits/logg"
)

////////////////////////////////////////////////////////////////////////////////
// type DatabaseSeed

// DatabaseSeed contains the contents of the seed file, if there is one.
type DatabaseSeed struct {
	Groups []GroupSeed `json:"groups"`
	Users  []UserSeed  `json:"users"`
}

// ReadDatabaseSeedFromEnvironment reads and validates the file at
// PORTUNUS_SEED_PATH. If that environment variable was not provided, nil is
// returned instead.
func ReadDatabaseSeedFromEnvironment(cfg *ValidationConfig) (*DatabaseSeed, errext.ErrorSet) {
	path := os.Getenv("PORTUNUS_SEED_PATH")
	if path == "" {
		return nil, nil
	}
	return ReadDatabaseSeed(path, cfg)
}

// ReadDatabaseSeed reads and validates the seed file at the given path.
func ReadDatabaseSeed(path string, cfg *ValidationConfig) (result *DatabaseSeed, errs errext.ErrorSet) {
	buf, err := os.ReadFile(path)
	if err != nil {
		errs.Add(err)
		return nil, errs
	}
	dec := json.NewDecoder(bytes.NewReader(buf))
	dec.DisallowUnknownFields()
	var seed DatabaseSeed
	err = dec.Decode(&seed)
	if err != nil {
		errs.Addf("while parsing %s: %w", path, err)
		return nil, errs
	}
	return &seed, seed.Validate(cfg)
}

// Validate returns an error if the seed contains any invalid or missing values.
func (d DatabaseSeed) Validate(cfg *ValidationConfig) (errs errext.ErrorSet) {
	//most validation can be performed by Database.Validate() by applying the
	//seed to a fresh database
	var db Database
	d.ApplyTo(&db, &NoopHasher{})
	errs = db.Validate(cfg)

	//the duplicate checks must be done differently for seeds because ApplyTo()
	//will not create duplicate users or groups
	groupNameCounts := make(map[string]int)
	for _, groupSeed := range d.Groups {
		groupNameCounts[string(groupSeed.Name)]++
	}
	for name, count := range groupNameCounts {
		if count > 1 {
			ref := Group{Name: name}.Ref()
			errs.Add(ref.Field("name").Wrap(errIsDuplicateInSeed))
		}
	}

	userLoginNameCounts := make(map[string]int)
	for _, userSeed := range d.Users {
		userLoginNameCounts[string(userSeed.LoginName)]++
	}
	for loginName, count := range userLoginNameCounts {
		if count > 1 {
			ref := User{LoginName: loginName}.Ref()
			errs.Add(ref.Field("login_name").Wrap(errIsDuplicateInSeed))
		}
	}

	//non-nil-ness of posix.uid and posix.gid on UserSeeds cannot be checked in
	//Database.Validate() because those fields are not pointers on type User
	for _, userSeed := range d.Users {
		ref := User{LoginName: string(userSeed.LoginName)}.Ref()
		if userSeed.POSIX != nil {
			if userSeed.POSIX.UID == nil {
				errs.Add(ref.Field("posix_uid").Wrap(errIsMissing))
			}
			if userSeed.POSIX.GID == nil {
				errs.Add(ref.Field("posix_gid").Wrap(errIsMissing))
			}
		}
	}

	return errs
}

// ApplyTo changes the given database to conform to the seed.
func (d DatabaseSeed) ApplyTo(db *Database, hasher crypt.PasswordHasher) {
	//for each group seed...
	for _, groupSeed := range d.Groups {
		//...either the group exists already...
		hasGroup := false
		for idx, group := range db.Groups {
			if group.Name == string(groupSeed.Name) {
				groupSeed.ApplyTo(&db.Groups[idx])
				hasGroup = true
				break
			}
		}

		//...or it needs to be created
		if !hasGroup {
			group := Group{Name: string(groupSeed.Name)}
			groupSeed.ApplyTo(&group)
			db.Groups = append(db.Groups, group)
		}
	}

	//same for the user seeds
	for _, userSeed := range d.Users {
		hasUser := false
		for idx, user := range db.Users {
			if user.LoginName == string(userSeed.LoginName) {
				userSeed.ApplyTo(&db.Users[idx], hasher)
				hasUser = true
				break
			}
		}
		if !hasUser {
			user := User{LoginName: string(userSeed.LoginName)}
			userSeed.ApplyTo(&user, hasher)
			db.Users = append(db.Users, user)
		}
	}

	db.Normalize()
}

var errSeededField = errors.New("must be equal to the seeded value")

// CheckConflicts returns errors for all ways in which the Database deviates
// from the seed's expectation.
func (d DatabaseSeed) CheckConflicts(db Database, hasher crypt.PasswordHasher) (errs errext.ErrorSet) {
	//if there are conflicts, then applying the seed to a copy of the DB will
	//result in a different DB -- we will call the original DB "left-hand side"
	//and its clone with the seed applied "right-hand side"
	leftDB := db
	rightDB := db.Cloned()
	d.ApplyTo(&rightDB, hasher) //includes Normalize

	//NOTE: We do not need to check for users/groups that exist on the left but
	//not on the right, because seeding only ever creates and updates objects,
	//but never deletes any objects.

	for _, rightGroup := range rightDB.Groups {
		leftGroup, exists := leftDB.Groups.Find(func(g Group) bool { return g.Name == rightGroup.Name })
		if !exists {
			errs.Addf("group %q is seeded and cannot be deleted", rightGroup.Name)
			continue
		}

		ref := leftGroup.Ref()
		if leftGroup.LongName != rightGroup.LongName {
			errs.Add(ref.Field("long_name").Wrap(errSeededField))
		}
		if leftGroup.Permissions.Portunus.IsAdmin != rightGroup.Permissions.Portunus.IsAdmin {
			errs.Add(ref.Field("portunus_perms").Wrap(errSeededField))
		}
		if leftGroup.Permissions.LDAP.CanRead != rightGroup.Permissions.LDAP.CanRead {
			errs.Add(ref.Field("ldap_perms").Wrap(errSeededField))
		}
		if !reflect.DeepEqual(leftGroup.PosixGID, rightGroup.PosixGID) {
			errs.Add(ref.Field("posix_gid").Wrap(errSeededField))
		}

		//NOTE: Same logic as above. Seeds only ever add group memberships and
		//never remove them, so we only need to check in one direction.
		for loginName, isRightMember := range rightGroup.MemberLoginNames {
			if isRightMember && !leftGroup.MemberLoginNames[loginName] {
				err := fmt.Errorf("must contain user %q because of seeded group membership", loginName)
				errs.Add(ref.Field("members").Wrap(err))
			}
		}
	}

	for _, rightUser := range rightDB.Users {
		leftUser, exists := leftDB.Users.Find(func(u User) bool { return u.LoginName == rightUser.LoginName })
		if !exists {
			errs.Addf("user %q is seeded and cannot be deleted", rightUser.LoginName)
			continue
		}

		ref := leftUser.Ref()
		if leftUser.GivenName != rightUser.GivenName {
			errs.Add(ref.Field("given_name").Wrap(errSeededField))
		}
		if leftUser.FamilyName != rightUser.FamilyName {
			errs.Add(ref.Field("family_name").Wrap(errSeededField))
		}
		if leftUser.EMailAddress != rightUser.EMailAddress {
			errs.Add(ref.Field("email").Wrap(errSeededField))
		}
		if !reflect.DeepEqual(leftUser.SSHPublicKeys, rightUser.SSHPublicKeys) {
			errs.Add(ref.Field("ssh_public_keys").Wrap(errSeededField))
		}
		if leftUser.PasswordHash != rightUser.PasswordHash {
			errs.Add(ref.Field("password").Wrap(errSeededField))
		}
		if (leftUser.POSIX == nil) != (rightUser.POSIX == nil) {
			errs.Add(ref.Field("posix").Wrap(errSeededField))
		}

		if leftUser.POSIX != nil && rightUser.POSIX != nil {
			leftPosix := *leftUser.POSIX
			rightPosix := *rightUser.POSIX
			if leftPosix.UID != rightPosix.UID {
				errs.Add(ref.Field("posix_uid").Wrap(errSeededField))
			}
			if leftPosix.GID != rightPosix.GID {
				errs.Add(ref.Field("posix_gid").Wrap(errSeededField))
			}
			if leftPosix.HomeDirectory != rightPosix.HomeDirectory {
				errs.Add(ref.Field("posix_home").Wrap(errSeededField))
			}
			if leftPosix.LoginShell != rightPosix.LoginShell {
				errs.Add(ref.Field("posix_shell").Wrap(errSeededField))
			}
			if leftPosix.GECOS != rightPosix.GECOS {
				errs.Add(ref.Field("posix_gecos").Wrap(errSeededField))
			}
		}
	}

	return errs
}

// Initializes the Database from the given seed on first use.
// If the seed is nil, the default initialization behavior is used.
func initializeDatabase(d *DatabaseSeed, hasher crypt.PasswordHasher) Database {
	//if no seed has been given, create the "admin" user with access to the
	//Portunus UI and log the password once
	if d == nil {
		password := hex.EncodeToString(GenerateRandomKey(16))
		passwordHash := hasher.HashPassword(password)
		logg.Info("first-time initialization: adding user %q with password %q",
			"admin", password)

		return Database{
			Groups: []Group{{
				Name:             "admins",
				LongName:         "Portunus Administrators",
				MemberLoginNames: GroupMemberNames{"admin": true},
				Permissions:      Permissions{Portunus: PortunusPermissions{IsAdmin: true}},
			}},
			Users: []User{{
				LoginName:    "admin",
				GivenName:    "Initial",
				FamilyName:   "Administrator",
				PasswordHash: passwordHash,
			}},
		}
	}

	//otherwise, initialize the DB from the seed
	var db Database
	d.ApplyTo(&db, hasher)
	return db
}

////////////////////////////////////////////////////////////////////////////////
// type GroupSeed

// GroupSeed contains the seeded configuration for a single group.
type GroupSeed struct {
	Name             StringSeed   `json:"name"`
	LongName         StringSeed   `json:"long_name"`
	MemberLoginNames []StringSeed `json:"members"`
	Permissions      struct {
		Portunus struct {
			IsAdmin *bool `json:"is_admin"`
		} `json:"portunus"`
		LDAP struct {
			CanRead *bool `json:"can_read"`
		} `json:"ldap"`
	} `json:"permissions"`
	PosixGID *PosixID `json:"posix_gid"`
}

// ApplyTo changes the attributes of this group to conform to the given seed.
func (g GroupSeed) ApplyTo(target *Group) {
	//consistency check (the caller must ensure that the seed matches the object)
	if target.Name != string(g.Name) {
		panic(fmt.Sprintf("cannot apply seed with Name = %q to group with Name = %q",
			string(g.Name), target.Name))
	}

	target.LongName = string(g.LongName)

	if target.MemberLoginNames == nil {
		target.MemberLoginNames = make(GroupMemberNames)
	}
	for _, loginName := range g.MemberLoginNames {
		target.MemberLoginNames[string(loginName)] = true
	}

	if g.Permissions.Portunus.IsAdmin != nil {
		target.Permissions.Portunus.IsAdmin = *g.Permissions.Portunus.IsAdmin
	}
	if g.Permissions.LDAP.CanRead != nil {
		target.Permissions.LDAP.CanRead = *g.Permissions.LDAP.CanRead
	}
	if g.PosixGID != nil {
		target.PosixGID = g.PosixGID
	}
}

////////////////////////////////////////////////////////////////////////////////
// type UserSeed

// UserSeed contains the seeded configuration for a single user.
type UserSeed struct {
	LoginName     StringSeed   `json:"login_name"`
	GivenName     StringSeed   `json:"given_name"`
	FamilyName    StringSeed   `json:"family_name"`
	EMailAddress  StringSeed   `json:"email"`
	SSHPublicKeys []StringSeed `json:"ssh_public_keys"`
	Password      StringSeed   `json:"password"`
	POSIX         *struct {
		UID           *PosixID   `json:"uid"`
		GID           *PosixID   `json:"gid"`
		HomeDirectory StringSeed `json:"home"`
		LoginShell    StringSeed `json:"shell"`
		GECOS         StringSeed `json:"gecos"`
	} `json:"posix"`
}

// ApplyTo changes the attributes of this group to conform to the given seed.
func (u UserSeed) ApplyTo(target *User, hasher crypt.PasswordHasher) {
	//consistency check (the caller must ensure that the seed matches the object)
	if target.LoginName != string(u.LoginName) {
		panic(fmt.Sprintf("cannot apply seed with LoginName = %q to user with LoginName = %q",
			string(u.LoginName), target.LoginName))
	}

	target.GivenName = string(u.GivenName)
	target.FamilyName = string(u.FamilyName)
	if u.EMailAddress != "" {
		target.EMailAddress = string(u.EMailAddress)
	}

	if len(u.SSHPublicKeys) > 0 {
		target.SSHPublicKeys = nil
		for _, key := range u.SSHPublicKeys {
			target.SSHPublicKeys = append(target.SSHPublicKeys, string(key))
		}
	}

	if u.Password != "" {
		//to avoid useless rehashing, the password is only applied:
		//- on creation (when no PasswordHash exists),
		//- on method mismatch (i.e. when the hasher wants us to change hash methods), or
		//- on password mismatch (i.e. when the password is updated in the seed)
		pw := string(u.Password)
		hash := target.PasswordHash
		if hash == "" || hasher.IsWeakHash(hash) || !hasher.CheckPasswordHash(pw, hash) {
			target.PasswordHash = hasher.HashPassword(pw)
		}
	}

	if u.POSIX != nil {
		if target.POSIX == nil {
			target.POSIX = &UserPosixAttributes{}
		}
		p := *u.POSIX
		//NOTE: The nil checks on p.UID and p.GID will never fire for valid
		//UserSeed objects, but we need to do them because this method is also
		//called during Validate() on possibly invalid UserSeed objects.
		if p.UID != nil {
			target.POSIX.UID = *p.UID
		}
		if p.GID != nil {
			target.POSIX.GID = *p.GID
		}
		target.POSIX.HomeDirectory = string(p.HomeDirectory)
		if p.LoginShell != "" {
			target.POSIX.LoginShell = string(p.LoginShell)
		}
		if p.GECOS != "" {
			target.POSIX.GECOS = string(p.GECOS)
		}
	}
}

////////////////////////////////////////////////////////////////////////////////
// type StringSeed

// StringSeed contains a single string value coming from the seed file.
type StringSeed string

// UnmarshalJSON implements the json.Unmarshaler interface.
func (s *StringSeed) UnmarshalJSON(buf []byte) error {
	//common case: unmarshal from string
	var val string
	err1 := json.Unmarshal(buf, &val)
	if err1 == nil {
		*s = StringSeed(val)
		return nil
	}

	//alternative case: perform command substitution
	var obj struct {
		Command []string `json:"from_command"`
	}
	err := json.Unmarshal(buf, &obj)
	if err != nil {
		//if this object syntax does not fit, return the original error where we
		//tried to unmarshal into a string value, since that probably makes more
		//sense in context
		return err1
	}
	if len(obj.Command) == 0 {
		return errors.New(`expected at least one entry in the "from_command" list`)
	}
	cmd := exec.Command(obj.Command[0], obj.Command[1:]...)
	cmd.Stdin = nil
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	*s = StringSeed(strings.TrimSuffix(string(out), "\n"))
	return nil
}
