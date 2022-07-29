/*******************************************************************************
*
* Copyright 2022 Stefan Majewsky <majewsky@gmx.net>
*
* This program is free software: you can redistribute it and/or modify it under
* the terms of the GNU General Public License as published by the Free Software
* Foundation, either version 3 of the License, or (at your option) any later
* version.
*
* This program is distributed in the hope that it will be useful, but WITHOUT ANY
* WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR
* A PARTICULAR PURPOSE. See the GNU General Public License for more details.
*
* You should have received a copy of the GNU General Public License along with
* this program. If not, see <http://www.gnu.org/licenses/>.
*
*******************************************************************************/

package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

////////////////////////////////////////////////////////////////////////////////
// type DatabaseSeed

//DatabaseSeed contains the contents of the seed file, if there is one.
type DatabaseSeed struct {
	Groups []GroupSeed `json:"groups"`
	Users  []UserSeed  `json:"users"`
}

//ReadDatabaseSeedFromEnvironment reads and validates the file at
//PORTUNUS_SEED_PATH. If that environment variable was not provided, nil is
//returned instead.
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

//Validate returns an error if the seed contains any invalid or missing values.
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

////////////////////////////////////////////////////////////////////////////////
// type GroupSeed

//GroupSeed contains the seeded configuration for a single group.
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

////////////////////////////////////////////////////////////////////////////////
// type UserSeed

//UserSeed contains the seeded configuration for a single user.
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

////////////////////////////////////////////////////////////////////////////////
// type StringSeed

//StringSeed contains a single string value coming from the seed file.
type StringSeed string

//UnmarshalJSON implements the json.Unmarshaler interface.
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
