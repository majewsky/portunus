/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package grammars

import (
	"regexp"
	"testing"
)

func FuzzIsLDAPSuffix(f *testing.F) {
	ldapSuffixRx := regexp.MustCompile(LDAPSuffixRegex)
	f.Add("dc=example,dc=com")
	f.Fuzz(func(t *testing.T, input string) {
		actual := IsLDAPSuffix(input)
		expected := ldapSuffixRx.MatchString(input)
		if actual != expected {
			t.Errorf("expected IsLDAPSuffix(%q) = %t, but got %t", input, expected, actual)
		}
	})
}

func FuzzIsListenAddress(f *testing.F) {
	listenAddressRx := regexp.MustCompile(ListenAddressRegex)
	f.Add(":8080")
	f.Add("127.0.0.1:53")
	f.Add("[::1]:1234")
	f.Fuzz(func(t *testing.T, input string) {
		actual := IsListenAddress(input)
		expected := listenAddressRx.MatchString(input)
		if actual != expected {
			t.Errorf("expected IsListenAddress(%q) = %t, but got %t", input, expected, actual)
		}
	})
}

func FuzzIsNonnegativeInteger(f *testing.F) {
	nonnegativeIntegerRx := regexp.MustCompile(NonnegativeIntegerRegex)
	f.Add("0")
	f.Fuzz(func(t *testing.T, input string) {
		actual := IsNonnegativeInteger(input)
		expected := nonnegativeIntegerRx.MatchString(input)
		if actual != expected {
			t.Errorf("expected IsNonnegativeInteger(%q) = %t, but got %t", input, expected, actual)
		}
	})
}

func FuzzIsPOSIXAccountName(f *testing.F) {
	posixAccountNameRx := regexp.MustCompile(POSIXAccountNameRegex)
	f.Add("john_doe")
	f.Add("bob$")
	f.Fuzz(func(t *testing.T, input string) {
		actual := IsPOSIXAccountName(input)
		expected := posixAccountNameRx.MatchString(input)
		if actual != expected {
			t.Errorf("expected IsPOSIXAccountName(%q) = %t, but got %t", input, expected, actual)
		}
	})
}
