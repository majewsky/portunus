/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import "strings"

// NoopHasher is a crypt.PasswordHasher that does not do any hashing. This
// dummy implementation is used when constructing temporary Database objects
// out of seeds, such as for seed validation, and also in unit tests.
type NoopHasher struct {
	UpgradeWeakHashes bool
}

// HashPassword implements the crypt.PasswordHasher interface.
func (n *NoopHasher) HashPassword(password string) string {
	return "{PLAINTEXT}" + password
}

// CheckPasswordHash implements the crypt.PasswordHasher interface.
func (n *NoopHasher) CheckPasswordHash(password, passwordHash string) bool {
	// besides the syntax that HashPassword generates, we accept the {WEAK-PLAINTEXT} prefix
	// with the same semantics, but that prefix will trigger rehashing
	for _, prefix := range []string{"{PLAINTEXT}", "{WEAK-PLAINTEXT}"} {
		if prefix+password == passwordHash {
			return true
		}
	}
	return false
}

// IsWeakHash implements the crypt.PasswordHasher interface.
func (n *NoopHasher) IsWeakHash(passwordHash string) bool {
	//Well, technically all hashes from this PasswordHasher are weak :)
	//But we specifically want to test the machinery for upgrading weak into strong hashes.
	return n.UpgradeWeakHashes && !strings.HasPrefix(passwordHash, "{PLAINTEXT}")
}
