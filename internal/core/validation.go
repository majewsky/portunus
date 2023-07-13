/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/crypto/ssh"
)

// this regexp copied from useradd(8) manpage
const posixAccountNamePattern = `[a-z_][a-z0-9_-]*\$?`

var (
	errIsMissing      = errors.New("is missing")
	errLeadingSpaces  = errors.New("may not start with a space character")
	errTrailingSpaces = errors.New("may not end with a space character")

	errNotPosixAccountName = errors.New("is not an acceptable user/group name matching the pattern /" + posixAccountNamePattern + "/")
	posixAccountNameRx     = regexp.MustCompile(`^` + posixAccountNamePattern + `$`)
	errNotPosixUIDorGID    = errors.New("is not a number between 0 and 65535 inclusive")

	errNotAbsolutePath = errors.New("must be an absolute path, i.e. start with a /")
)

// MustNotBeEmpty is a h.ValidationRule.
func MustNotBeEmpty(val string) error {
	if strings.TrimSpace(val) == "" {
		return errIsMissing
	}
	return nil
}

// MustNotHaveSurroundingSpaces is a h.ValidationRule.
func MustNotHaveSurroundingSpaces(val string) error {
	if val != "" {
		if strings.TrimLeftFunc(val, unicode.IsSpace) != val {
			return errLeadingSpaces
		}
		if strings.TrimRightFunc(val, unicode.IsSpace) != val {
			return errTrailingSpaces
		}
	}
	return nil
}

// MustBePosixAccountName is a h.ValidationRule.
func MustBePosixAccountName(val string) error {
	if posixAccountNameRx.MatchString(val) {
		return nil
	}
	return errNotPosixAccountName
}

// MustBePosixUIDorGID is a h.ValidationRule.
func MustBePosixUIDorGID(val string) error {
	if val != "" {
		_, err := strconv.ParseUint(val, 10, 16)
		if err != nil {
			return errNotPosixUIDorGID
		}
	}
	return nil
}

// MustBeAbsolutePath is a h.ValidationRule.
func MustBeAbsolutePath(val string) error {
	if val != "" && !strings.HasPrefix(val, "/") {
		return errNotAbsolutePath
	}
	return nil
}

// SplitSSHPublicKeys preprocesses the content of a submitted <textarea> where a
// list of SSH public keys is expected. The result will have one public key per
// array entry.
func SplitSSHPublicKeys(val string) (result []string) {
	for _, line := range strings.Split(val, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// MustBeSSHPublicKeys is a h.ValidationRule.
func MustBeSSHPublicKeys(val string) error {
	for idx, line := range SplitSSHPublicKeys(val) {
		_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
		if err != nil {
			return fmt.Errorf("must have a valid SSH public key on each line (parse error on line %d)", idx+1)
		}
	}
	return nil
}

// MustBeSSHPublicKey is a h.ValidationRule.
func MustBeSSHPublicKey(val string) error {
	_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(val))
	if err != nil {
		return errors.New("must be a valid SSH public key")
	}
	return nil
}
