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

func TestGrammars(t *testing.T) {
	var testCases = []string{
		// valid LDAP suffixes
		"dc=example,dc=com",
		"dc=net",
		"dc=1,dc=example,dc=org",
		// invalid LDAP suffixes
		"",
		",dc=example,dc=com",         //empty segment
		"dc=example,dc=com,",         //empty segment
		"ou=users,dc=example,dc=com", //only dc= allowed
		"=example,dc=com",            //empty key
		"dc=example,dc=",             //empty value
		"dc=example!,dc=com",         //invalid chars in value
		"dc=ldap,dc=example.com",     //invalid chars in value
		"example,dc=com",             //missing key

		//valid listen addresses
		"1.2.3.4:5",
		"151587081:53", //single-number IP notation (same as 9.9.9.9:53)
		"[::1]:8000",
		"[2001:db08:ac10:fe01::]:8000",
		//invalid listen addresses
		"",
		":8080",          //our grammar requires an explicit IP
		"1.2.3.4",        //no port
		"1.2.3.4:",       //empty port
		"example.com:53", //hostnames are not allowed
		"[:0",            //found by fuzzing

		//valid POSIX account names
		"john_doe",
		"john-doe",
		"bob$",
		"user17",
		//invalid POSIX account names
		"",
		"JohnDoe",  //uppercase not allowed
		"john.doe", //dot not allowed
		"jöhn_döe", //Unicode chars not allowed
		"$bob",     //dollar only allowed at end
		"-johndoe", //dash not allowed at start
		"17users",  //number not allowed at start
		"a-",       //found by fuzzing

		//valid non-negative integers
		"0",
		"42",
		"503",
		//invalid non-negative integers
		"",
		"00",  //superfluous zeroes
		"080", //leading zero
		"0.5", //invalid characters
		"a+2", //invalid characters
	}

	// The test checks that the Is...() functions return the same results as
	// their defining regexes.
	ldapSuffixRx := regexp.MustCompile(LDAPSuffixRegex)
	listenAddressRx := regexp.MustCompile(ListenAddressRegex)
	nonnegativeIntegerRx := regexp.MustCompile(NonnegativeIntegerRegex)
	posixAccountNameRx := regexp.MustCompile(POSIXAccountNameRegex)

	for _, input := range testCases {
		actual := IsLDAPSuffix(input)
		expected := ldapSuffixRx.MatchString(input)
		if actual != expected {
			t.Errorf("expected IsLDAPSuffix(%q) = %t, but got %t", input, expected, actual)
		}

		actual = IsListenAddress(input)
		expected = listenAddressRx.MatchString(input)
		if actual != expected {
			t.Errorf("expected IsListenAddress(%q) = %t, but got %t", input, expected, actual)
		}

		actual = IsNonnegativeInteger(input)
		expected = nonnegativeIntegerRx.MatchString(input)
		if actual != expected {
			t.Errorf("expected IsPOSIXAccountName(%q) = %t, but got %t", input, expected, actual)
		}

		actual = IsPOSIXAccountName(input)
		expected = posixAccountNameRx.MatchString(input)
		if actual != expected {
			t.Errorf("expected IsPOSIXAccountName(%q) = %t, but got %t", input, expected, actual)
		}
	}
}
