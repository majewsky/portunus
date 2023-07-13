/*******************************************************************************
* Copyright 2022 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gorilla/securecookie"
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
func ReadDatabaseSeedFromEnvironment() (*DatabaseSeed, error) {
	path := os.Getenv("PORTUNUS_SEED_PATH")
	if path == "" {
		return nil, nil
	}
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var seed DatabaseSeed
	err = json.Unmarshal(buf, &seed)
	if err != nil {
		return nil, err
	}
	return &seed, seed.Validate()
}

// Validate returns an error if the seed contains any invalid or missing values.
func (d DatabaseSeed) Validate() error {
	isUserLoginName := make(map[string]bool)
	for idx, u := range d.Users {
		err := u.validate(isUserLoginName)
		if err != nil {
			return fmt.Errorf("seeded user #%d (%q) is invalid: %w", idx+1, u.LoginName, err)
		}
	}

	isGroupName := make(map[string]bool)
	for idx, g := range d.Groups {
		err := g.validate(isUserLoginName, isGroupName)
		if err != nil {
			return fmt.Errorf("seeded group #%d (%q) is invalid: %w", idx+1, g.Name, err)
		}
	}

	return nil
}

// DatabaseInitializer returns a function that initalizes the Database from the
// given seed on first use. If the seed is nil, the default initialization
// behavior is used.
func DatabaseInitializer(d *DatabaseSeed) func() Database {
	//if no seed has been given, create the "admin" user with access to the
	//Portunus UI and log the password once
	if d == nil {
		return func() Database {
			password := hex.EncodeToString(securecookie.GenerateRandomKey(16))
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
					PasswordHash: HashPasswordForLDAP(password),
				}},
			}
		}
	}

	//otherwise, initialize the DB from the seed
	return func() (db Database) {
		for _, userSeed := range d.Users {
			user := User{LoginName: string(userSeed.LoginName)}
			userSeed.ApplyTo(&user)
			db.Users = append(db.Users, user)
		}
		for _, groupSeed := range d.Groups {
			group := Group{Name: string(groupSeed.Name)}
			groupSeed.ApplyTo(&group)
			db.Groups = append(db.Groups, group)
		}
		return
	}
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

func (g GroupSeed) validate(isUserLoginName, isGroupName map[string]bool) error {
	err := g.Name.validate("name",
		MustNotBeEmpty,
		MustNotHaveSurroundingSpaces,
		MustBePosixAccountName,
	)
	if err != nil {
		return err
	}

	if isGroupName[string(g.Name)] {
		return errors.New("duplicate name")
	}
	isGroupName[string(g.Name)] = true

	err = g.LongName.validate("long_name",
		MustNotBeEmpty,
		MustNotHaveSurroundingSpaces,
	)
	if err != nil {
		return err
	}

	for _, loginName := range g.MemberLoginNames {
		if !isUserLoginName[string(loginName)] {
			return fmt.Errorf("group member %q is not defined in the seed", string(loginName))
		}
	}

	return nil
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

func (u UserSeed) validate(isUserLoginName map[string]bool) error {
	err := u.LoginName.validate("login_name",
		MustNotBeEmpty,
		MustNotHaveSurroundingSpaces,
		MustBePosixAccountName,
	)
	if err != nil {
		return err
	}

	if isUserLoginName[string(u.LoginName)] {
		return errors.New("duplicate login name")
	}
	isUserLoginName[string(u.LoginName)] = true

	err = u.GivenName.validate("given_name",
		MustNotBeEmpty,
		MustNotHaveSurroundingSpaces,
	)
	if err != nil {
		return err
	}

	err = u.FamilyName.validate("family_name",
		MustNotBeEmpty,
		MustNotHaveSurroundingSpaces,
	)
	if err != nil {
		return err
	}

	err = u.EMailAddress.validate("email",
		MustNotHaveSurroundingSpaces,
	)
	if err != nil {
		return err
	}

	for idx, sshPublicKey := range u.SSHPublicKeys {
		err := sshPublicKey.validate(fmt.Sprintf("ssh_public_keys[%d]", idx),
			MustNotBeEmpty,
			MustBeSSHPublicKey,
		)
		if err != nil {
			return err
		}
	}

	if u.POSIX != nil {
		if u.POSIX.UID == nil {
			return fmt.Errorf("posix.uid is missing")
		}
		if u.POSIX.GID == nil {
			return fmt.Errorf("posix.gid is missing")
		}

		err = u.POSIX.HomeDirectory.validate("posix.home",
			MustNotBeEmpty,
			MustNotHaveSurroundingSpaces,
			MustBeAbsolutePath,
		)
		if err != nil {
			return err
		}

		err = u.POSIX.LoginShell.validate("posix.shell",
			MustBeAbsolutePath,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// ApplyTo changes the attributes of this group to conform to the given seed.
func (u UserSeed) ApplyTo(target *User) {
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
		//Password is only applied on creation (when no PasswordHash exists) or on
		//password mismatch, otherwise we avoid useless rehashing
		pw := string(u.Password)
		if target.PasswordHash == "" || !CheckPasswordHash(pw, target.PasswordHash) {
			target.PasswordHash = HashPasswordForLDAP(pw)
		}
	}

	if u.POSIX != nil {
		if target.POSIX == nil {
			target.POSIX = &UserPosixAttributes{}
		}
		p := *u.POSIX
		target.POSIX.UID = *p.UID
		target.POSIX.GID = *p.GID
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

func (s StringSeed) validate(field string, rules ...func(string) error) error {
	for _, rule := range rules {
		err := rule(string(s))
		if err != nil {
			return fmt.Errorf("%s %w", field, err)
		}
	}
	return nil
}
