/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

// Package grammars contains explicit implementations of several regex grammars.
//
// These implementations are used to avoid pulling the entire regex engine into
// the portunus-orchestrator binary.
package grammars

import (
	"strings"
)

//TODO: reevaluate LDAPSuffixRegex against current DNS RFCs

const (
	// LDAPSuffixRegex is a regex for matching LDAP suffixes like `dc=example,dc=com`.
	//
	// This is only shown for documentation purposes here; use func IsLDAPSuffix instead.
	LDAPSuffixRegex = `^dc=[a-z0-9_-]+(?:,dc=[a-z0-9_-]+)*$`

	// ListenAddressRegex is a regex for matching listen addresses (pairs of IP
	// addresses and port numbers) like `1.2.3.4:55` or `[::1]:8000`. Note that
	// IP addresses and port numbers are not fully parsed; this is only a sanity
	// check to find absolutely invalid characters.
	//
	// This is only shown for documentation purposes here; use func IsListenAddress instead.
	ListenAddressRegex = `^(?:[0-9.]+|\[[0-9a-f:]+\]):[0-9]+$`

	// NonnegativeIntegerRegex is a regex for matching non-negative integers.
	//
	// This is only shown for documentation purposes here; use func IsNonnegativeInteger instead.
	NonnegativeIntegerRegex = `^(?:0|[1-9][0-9]*)$`

	// POSIXAccountNameRegex is a regex for matching POSIX user or group names.
	// This regex is based on the respective format description in the useradd(8) manpage.
	//
	// This is only shown for documentation purposes here; use func IsPOSIXAccountName instead.
	POSIXAccountNameRegex = `^[a-z_][a-z0-9_-]*\$?$`
)

//TODO There is also some `import "regexp"` in cmd/orchestrator/ldap.go to render
//the LDAP config.

// IsLDAPSuffix returns whether the string matches LDAPSuffixRegex.
func IsLDAPSuffix(input string) bool {
	for _, field := range strings.Split(input, ",") {
		key, value, found := strings.Cut(field, "=")
		if !found {
			return false
		}
		if key != "dc" {
			return false
		}
		if len(value) == 0 {
			return false
		}
		if !checkEachByte([]byte(value), checkByteInDomainComponent) {
			return false
		}
	}
	return true
}

func checkByteInDomainComponent(idx, length int, b byte) bool {
	_ = length
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b >= '0' && b <= '9':
		return true
	case b == '-' || b == '_':
		return true
	default:
		return false
	}
}

// IsListenAddress returns whether the string matches ListenAddressRegex.
func IsListenAddress(input string) bool {
	sepIndex := strings.LastIndexByte(input, ':')
	if sepIndex == -1 {
		return false
	}
	ipAddressInput := []byte(input[0:sepIndex])
	portNumberInput := []byte(input[sepIndex+1:])
	if len(ipAddressInput) == 0 || len(portNumberInput) == 0 {
		return false
	}
	if !checkEachByte(portNumberInput, checkByteInPortNumber) {
		return false
	}
	if ipAddressInput[0] == '[' {
		if len(ipAddressInput) < 3 {
			return false
		}
		return checkEachByte(ipAddressInput, checkByteInIPv6Address)
	} else {
		return checkEachByte(ipAddressInput, checkByteInIPv4Address)
	}
}

func checkByteInIPv4Address(idx, length int, b byte) bool {
	_, _ = idx, length
	return (b >= '0' && b <= '9') || b == '.'
}

func checkByteInIPv6Address(idx, length int, b byte) bool {
	switch idx {
	case 0:
		return b == '['
	case length - 1:
		return b == ']'
	default:
		return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || b == ':'
	}
}

func checkByteInPortNumber(idx, length int, b byte) bool {
	_, _ = idx, length
	return b >= '0' && b <= '9'
}

// IsNonnegativeInteger returns whether the string matches NonnegativeIntegerRegex.
func IsNonnegativeInteger(input string) bool {
	if len(input) == 0 {
		return false
	}
	if input == "0" {
		return true
	}
	return checkEachByte([]byte(input), checkByteInPositiveInteger)
}

func checkByteInPositiveInteger(idx, length int, b byte) bool {
	switch {
	case b == '0':
		return idx != 0 // not allowed at start (the value 0 is not allowed here)
	case b >= '1' && b <= '9':
		return true
	default:
		return false
	}
}

// IsPOSIXAccountName returns whether the string matches POSIXAccountNameRegex.
func IsPOSIXAccountName(input string) bool {
	input = strings.TrimSuffix(input, "$")
	if len(input) == 0 {
		return false
	}
	return checkEachByte([]byte(input), checkByteInPOSIXAccountName)
}

func checkByteInPOSIXAccountName(idx, length int, b byte) bool {
	switch {
	case (b >= 'a' && b <= 'z') || b == '_':
		return true
	case (b >= '0' && b <= '9') || b == '-':
		return idx != 0 // not allowed at start
	default:
		return false
	}
}

// Helper function: Returns whether each byte in the input is accepted by `check`.
func checkEachByte(bytes []byte, check func(idx, length int, b byte) bool) bool {
	l := len(bytes)
	for idx, b := range bytes {
		if !check(idx, l, b) {
			return false
		}
	}
	return true
}
