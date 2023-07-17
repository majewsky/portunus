/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import (
	"strings"

	"github.com/gorilla/securecookie"
	"github.com/majewsky/portunus/internal/shared"
	"github.com/sapcc/go-bits/logg"
	"github.com/tredoe/osutil/user/crypt"
	"github.com/tredoe/osutil/user/crypt/sha256_crypt"
)

var bogusPasswordHash = shared.HashPasswordForLDAP(string(securecookie.GenerateRandomKey(32)))

// CheckPasswordHash verifies the given password in nearly constant time.
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
