/*******************************************************************************
*
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
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
	"strings"

	"github.com/gorilla/securecookie"
	"github.com/sapcc/go-bits/logg"
	"github.com/tredoe/osutil/user/crypt"
	"github.com/tredoe/osutil/user/crypt/sha256_crypt"
	goldap "github.com/go-ldap/ldap/v3"
)

//HashPasswordForLDAP produces a password hash in the format expected by LDAP,
//like the libc function crypt(3).
func HashPasswordForLDAP(password string) string {
	//according to documentation, Crypter.Generate() will never return any errors
	//when the second argument is nil
	result, _ := sha256_crypt.New().Generate([]byte(password), nil)
	return "{CRYPT}" + result
}

var bogusPasswordHash = HashPasswordForLDAP(string(securecookie.GenerateRandomKey(32)))

//CheckPasswordHash verifies the given password in nearly constant time.
func CheckPasswordHash(password, passwordHash string) bool {
	//When this method is called on a non-existing user, i.e. the passwordHash is
	//the empty string, do not leak this fact to unauthorized users through the
	//timing side channel. Instead, we check the input against a bogus password
	//to take a comparable amount of time, then return false regardless.
	userExists := true
	if passwordHash == "" {
		userExists = false
		passwordHash = bogusPasswordHash
	}
	err := sha256_crypt.New().Verify(
		strings.TrimPrefix(passwordHash, "{CRYPT}"),
		[]byte(password))
	switch err {
	case nil:
		return userExists
	case crypt.ErrKeyMismatch:
		return false
	default:
		logg.Error("error in password verification: " + err.Error())
		return false
	}
}

func mkAttr(typeName string, values ...string) goldap.Attribute {
	return goldap.Attribute{Type: typeName, Vals: values}
}
