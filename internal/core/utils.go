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
	"os"
	"regexp"

	"github.com/sapcc/go-bits/logg"
	"github.com/tredoe/osutil/user/crypt/sha256_crypt"
)

//HashPasswordForLDAP produces a password hash in the format expected by LDAP,
//like the libc function crypt(3).
func HashPasswordForLDAP(password string) string {
	//according to documentation, Crypter.Generate() will never return any errors
	//when the second argument is nil
	result, _ := sha256_crypt.New().Generate([]byte(password), nil)
	return "{CRYPT}" + result
}

//GetenvResult is a helper type returned by Getenv().
type GetenvResult struct {
	Key   string
	Value string
}

//Getenv wraps os.Getenv with a nicer interface.
func Getenv(key string) GetenvResult {
	return GetenvResult{key, os.Getenv(key)}
}

//Format checks that the variable's value matches the given format, and
//produces a fatal error if not.
func (r GetenvResult) Format(rx *regexp.Regexp) GetenvResult {
	if r.Value != "" && !rx.MatchString(r.Value) {
		logg.Fatal("malformed environment variable: %s must look like /%s/", r.Value, rx.String())
	}
	return r
}

//Must returns the variable's value, or produces a fatal error if it was not set.
func (r GetenvResult) Must() string {
	if r.Value == "" {
		logg.Fatal("missing required environment variable: " + r.Key)
	}
	return r.Value
}

//Or returns the variable's value, or the given default value if it was not set.
func (r GetenvResult) Or(defaultValue string) string {
	if r.Value == "" {
		return defaultValue
	}
	return r.Value
}
