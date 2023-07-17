/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

// Package shared contains code that is imported by cmd/orchestrator.
//
// Please keep this package as small as reasonably possible, esp. with regards
// to imports. All the code that is located or imported here ends up in the
// orchestrator binary that runs with root privileges. Let's all do our part to
// keep the TCB small.
package shared

import "github.com/tredoe/osutil/user/crypt/sha256_crypt"

// HashPasswordForLDAP produces a password hash in the format expected by LDAP,
// like the libc function crypt(3).
func HashPasswordForLDAP(password string) string {
	//according to documentation, Crypter.Generate() will never return any errors
	//when the second argument is nil
	result, _ := sha256_crypt.New().Generate([]byte(password), nil)
	return "{CRYPT}" + result
}
